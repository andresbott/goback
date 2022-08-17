COMMIT_SHA_SHORT ?= $(shell git rev-parse --short=12 HEAD)
PWD_DIR := ${CURDIR}

# Check that given variables are set and all have non-empty values,
# die with an error otherwise.
#
# Params:
#   1. Variable name(s) to test.
#   2. (optional) Error message to print.
check_defined = \
    $(strip $(foreach 1,$1, \
        $(call __check_defined,$1,$(strip $(value 2)))))
__check_defined = \
    $(if $(value $1),, \
      $(error Undefined attribute $1$(if $2, ($2))))
# call with:
#		@:$(call check_defined, TAG, Tag value for docker image)


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
	@:$(call check_defined, version, "version defined: call with version=\"v1.2.3\"")
	@git diff --quiet || ( echo 'git is in dirty state' ; exit 1 )
	@[ "${version}" ] || ( echo ">> version is not set, usage: make release version=\"v1.2.3\" "; exit 1 )
	@git tag -d $(version) || true # delete tag if it exists, allows to overwrite tags
	@git push --delete origin $(version) || true
	@git tag -a $(version) -m "Release version: $(version)"
	@git push origin $(version)
	@goreleaser --rm-dist

clean: ## clean the build environment
	@rm -rf ./dist

help: ## help command
	@egrep '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST)  | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

