RELEASE_SNAPSHOT_VERSION ?= 0.0.0-SNAPSHOT

default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	go generate .
	cd tools; GOWORK=off go generate ./...

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

release-snapshot:
	VERSION=$(RELEASE_SNAPSHOT_VERSION) ./scripts/build-release-artifacts.sh

.PHONY: fmt lint test testacc build install generate release-snapshot
