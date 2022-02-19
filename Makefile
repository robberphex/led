MDs := $(shell find . -name '*.md')
HTMLs := $(MDs:.md=.html)
ROOT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

%.html: %.md ./head.tpl ./article.tpl
	pandoc -s -p --from gfm --highlight-style=pygments \
		--template article.tpl \
		--metadata-file=index.yaml \
		--lua-filter $(ROOT_DIR)/description.lua \
		$< -o $@

index:
	find . -type d ! -path '*.assets' -exec index.sh {} \;

all: index $(HTMLs)
	feed.sh . > feed.xml
	map.sh . > map.xml
