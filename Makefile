COMMIT_SHA_SHORT ?= $(shell git rev-parse --short=12 HEAD)
PWD_DIR := ${CURDIR}

default: help

status: ## get info about the project
	@echo "current commit: ${COMMIT_SHA_SHORT}"

fmt: ## format go code and run mod tidy
	@go fmt ./...
	@go mod tidy

.PHONY: test
test: ## run go tests
	@go test ./... -v -cover

lint: ## run go linter
	@golangci-lint run

verify: fmt test benchmark lint ## run all verification and code structure tiers

benchmark: ## run go benchmarks
	@go test -run=^$$ -bench=. ./...

build: ## builds a snapshot build using goreleaser
	@goreleaser --snapshot --rm-dist

release: verify ## release a new version of goback
	@git diff --quiet || ( echo 'git is in dirty state' ; exit 1 )
	@[ "${version}" ] || ( echo ">> version is not set, usage: make release version=\"v1.2.3\" "; exit 1 )
	@git tag -a $(version) -m "Release version: $(version)"
	@git push origin $(version)
	@goreleaser --rm-dist


help: ## help command
	@egrep '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST)  | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

