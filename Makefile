GITHUB_EMAIL ?= foo@form3.tech
GITHUB_USERNAME ?= foo

default : vet test build

.PHONY: build
build:
	go build

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	GITHUB_EMAIL=$(GITHUB_EMAIL) \
	GITHUB_USERNAME=$(GITHUB_USERNAME) \
	go test -v ./...
