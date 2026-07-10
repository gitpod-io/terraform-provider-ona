RELEASE_SNAPSHOT_VERSION ?= 0.0.0-SNAPSHOT

default: fmt lint install generate

build:
	go build -v ./...

install-dependencies:
	go mod download
	cd tools; GOWORK=off go mod download

install: build
	go install -v ./...

fmt: fmt-go fmt-terraform

fmt-go:
	gofmt -s -w -e .

fmt-terraform:
	terraform fmt -recursive examples/ dev/local-devloop/

lint:
	golangci-lint run

generate:
	cd tools; GOWORK=off go generate tools.go

test: test-unit test-acc

test-unit:
	go test -v -cover -timeout=120s -parallel=10 ./...

test-acc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

release-snapshot:
	VERSION=$(RELEASE_SNAPSHOT_VERSION) ./scripts/build-release-artifacts.sh

.PHONY: fmt fmt-go fmt-terraform lint test test-unit test-acc build install-dependencies install generate release-snapshot
