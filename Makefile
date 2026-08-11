VERSION ?= 0.1.0
BINARY ?= silo-plugin-sync-watcharr

.PHONY: test build manifest tidy

tidy:
	go mod tidy

test:
	go test ./...

build:
	mkdir -p bin
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY) .

manifest: build
	./bin/$(BINARY) manifest
