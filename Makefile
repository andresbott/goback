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

##@ Test
fmt: ## format go code and run mod tidy
	@go fmt ./...
	@go mod tidy

.PHONY: test
test: ## run go tests
	@go test ./... -cover

test-long: ## run go tests, long run
	@RUN_TESTCONTAINERS=true go test ./... -cover

lint: ## run go linter
	@golangci-lint run

verify: fmt test-long lint ## run all verification and code structure tiers


##@ Build
build: ## builds a snapshot build using goreleaser
	@goreleaser release --clean --snapshot

clean: ## clean the build environment
	@rm -rf ./dist


##@ Release

check_env: # check for needed envs
ifndef GITHUB_TOKEN
	$(error GITHUB_TOKEN is undefined, create one with repo permissions here: https://github.com/settings/tokens/new?scopes=repo,write:packages)
endif
	@[ "${version}" ] || ( echo ">> version is not set, usage: make release version=\"v1.2.3\" "; exit 1 )

.PHONY: check-git-clean
check-git-clean: # check if git repo is clen
	@git diff --quiet

.PHONY: check-branch
check-branch:
	@current_branch=$$(git symbolic-ref --short HEAD) && \
	if [ "$$current_branch" != "main" ]; then \
		echo "Error: You are on branch '$$current_branch'. Please switch to 'main'."; \
		exit 1; \
	fi

release: check_env check-branch check-git-clean verify ## release a new version, call with version="v1.2.3", make sure to have valid GH token
	@[ "${version}" ] || ( echo ">> version is not set, mausage: make release version=\"v1.2.3\" "; exit 1 )
	@git tag -d $(version) || true
	@git tag -a $(version) -m "Release version: $(version)"
	@git push --delete origin $(version) || true
	@git push origin $(version) || true
	@goreleaser release --clean


##@ Help
.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
