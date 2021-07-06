package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/lvht/ssltun"
	"golang.org/x/crypto/acme/autocert"
)

var name, key, root string
var h2 bool
var h3 string

func init() {
	flag.StringVar(&name, "name", "", "server domain name")
	flag.StringVar(&key, "key", "", "server auth key")
	flag.StringVar(&root, "root", "", "static server root")
	flag.StringVar(&h3, "h3", "", "h3 listen port")
	flag.BoolVar(&h2, "h2", false, "enable http/2 protocol")
}

func main() {
	flag.Parse()
	if name == "" || key == "" {
		flag.Usage()
		return
	}

	names := strings.Split(name, ",")

	dir := os.Getenv("HOME") + "/.autocert"
	acm := autocert.Manager{
		Cache:      autocert.DirCache(dir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(names...),
	}
	tlsCfg := acm.TLSConfig()

	if !h2 {
		tlsCfg.NextProtos = []string{"http/1.1", "acme-tls/1"}
	}

	ln, err := tls.Listen("tcp", ":443", tlsCfg)
	if err != nil {
		log.Fatal(err)
	}

	proxy := &ssltun.Proxy{DomainNames: names}
	proxy.Auth = func(u, p string) bool { return u == key }
	if root != "" {
		proxy.FileHandlers = make(map[string]ssltun.Handler, len(names))
		for _, name := range names {
			path := filepath.Join(root, name)

			h := http.FileServer(http.Dir(path))
			h = handlers.CombinedLoggingHandler(os.Stdout, h)
			h = handlers.CompressHandler(h)
			proxy.FileHandlers[name] = ssltun.Handler{
				Root:    path,
				Handler: h,
			}
		}
	}

	go func() {
		if h3 == "" {
			return
		}

		tlsCfg := acm.TLSConfig()
		tlsCfg.NextProtos = []string{"h3"}

		ln, err := net.ListenPacket("udp", ":"+h3)
		if err != nil {
			log.Fatal(err)
		}

		h3p := proxy

		for _, p := range tlsCfg.NextProtos {
			proxy.AltSvc = append(proxy.AltSvc, p+`=":`+h3+`"`)
		}

		h3 := http3.Server{Server: &http.Server{Handler: h3p}}
		h3.TLSConfig = tlsCfg
		h3.Serve(ln)
	}()

	go func() {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			url := "https://" + r.Host + r.RequestURI
			http.Redirect(w, r, url, http.StatusMovedPermanently)
		})

		if err := http.ListenAndServe(":80", h); err != nil {
			log.Fatal(err)
		}
	}()

	if err := http.Serve(ln, proxy); err != nil {
		log.Fatal(err)
	}
}
