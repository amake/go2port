SHELL := /bin/bash
version := dev

.PHONY: all
all: help

.PHONY: build
build: ## Build the go2port binary
build: go2port

go2port:
	go build -ldflags '-X main.version=$(version)'

.PHONY: clean
clean: ## Remove generated files
	rm -f go2port

.PHONY: test
test: ## Run tests
test: test-bin test-get

.PHONY: test-bin
test-bin: | go2port
	@[[ $$(./go2port -v) = "go2port version dev" ]] && echo ok

.PHONY: test-get
test-get: | go2port
	@diff <(./go2port get github.com/amake/go2port 3e11cdb) test/gold/Portfile.go2port && echo ok

.PHONY: help
help: ## Show this help text
	$(info usage: make [target])
	$(info )
	$(info Available targets:)
	@awk -F ':.*?## *' '/^[^\t].+?:.*?##/ \
         {printf "  %-24s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
