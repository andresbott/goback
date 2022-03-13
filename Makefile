COMMIT_SHA_SHORT ?= $(shell git rev-parse --short=12 HEAD)
PWD_DIR := ${CURDIR}

default: help

status: ## get info about the project
	@echo "current commit: ${COMMIT_SHA_SHORT}"

fmt: ## format go code and run mod tidy
	@go fmt ./...
	@go mod tidy

test: ## run go tests
	@go test ./... -v -cover

lint: ## run go linter
	@golangci-lint run

verify: fmt test benchmark lint ## run all verification and code structure tiers

benchmark: ## run go benchmarks
	@go test -run=^$$ -bench=. ./...

build: ## builds a snapshot build using goreleaser
	@goreleaser --snapshot --rm-dist


help: ## help command
	@egrep '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST)  | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'


docker-e2e: ## run end to end tests in a docker container
	@docker build -t goback-test-$(COMMIT_SHA_SHORT) -f zarf/docker/DockerFile .
	@docker run -e MARIADB_ROOT_PASSWORD=admin E2E_TEST=true goback-test-$(COMMIT_SHA_SHORT) go test -v app/e2e/e2e_test.go


