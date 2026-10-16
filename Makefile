.PHONY: build test fmt

build:
	go build -o plumelog .

test:
	go test ./...

fmt:
	gofmt -w .
