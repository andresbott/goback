# GoBack

[![CircleCI](https://circleci.com/gh/AndresBott/goback/tree/main.svg?style=svg)](https://circleci.com/gh/AndresBott/goback/tree/main)

## roadmap

* separete profile settings for remote dir back from remote sync 
* backup git repositories
  
* improve cli output
* add -quite flag
* write documentation

### Development

#### Requirements

* go
* make
* docker
* goreleaser
* golangci-lint
* git 

#### Release

make sure you have your gh token stored locally in ~/.goreleaser/gh_token

to release a new version:
```bash 
make release  version="v0.1.2"
```
