default : vet test build

.PHONY: build
build:
	go build

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test -v ./...
