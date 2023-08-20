SHELL := /bin/bash
version := dev

.PHONY: all
all: help

.PHONY: build
build: ## Build the go2port binary
build: go2port

go2port: $(wildcard *.go)
	go build -ldflags '-X main.version=$(version)'

.PHONY: clean
clean: ## Remove generated files
	rm -rf go2port test/tmp

.PHONY: test
test: ## Run tests
test: test-bin test-get test-update

.PHONY: test-bin
test-bin: | go2port
	@[[ $$(./go2port -v) = "go2port version $(version)" ]] && echo bin: ok

.PHONY: test-get
test-get: | go2port
	@diff <(./go2port get github.com/amake/go2port 6d6dc46) test/gold/get/Portfile.go2port.6d6dc46 && echo glide.lock: ok
	@diff <(./go2port get github.com/amake/go2port c72df06) test/gold/get/Portfile.go2port.c72df06 && echo Gopkg.lock: ok
	@diff <(./go2port get github.com/amake/go2port 3e11cdb) test/gold/get/Portfile.go2port.3e11cdb && echo go.sum: ok

.PHONY: test-update
test-update: | go2port
	@PATH="$(PWD)/test/bin:$$PATH" ./go2port update go2port 3e11cdb
	@diff test/tmp/Portfile test/gold/update/Portfile.go2port.3e11cdb && echo update: ok

.PHONY: help
help: ## Show this help text
	$(info usage: make [target])
	$(info )
	$(info Available targets:)
	@awk -F ':.*?## *' '/^[^\t].+?:.*?##/ \
         {printf "  %-24s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
