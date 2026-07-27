RELEASE_SNAPSHOT_VERSION ?= 0.0.0-SNAPSHOT

# Discover only checked-in modules, excluding scratch and dependency cache trees.
GO_MODULE_DIRS := $(shell git ls-files --cached -- '*go.mod' | \
	awk '!/(^|\/)(\.git|\.tmp|\.cache|cache|vendor|node_modules)(\/|$$)/' | \
	sed -e 's#/go.mod$$##' -e 's#^go.mod$$#.#' | sort)

default: fmt lint install generate

define run-in-go-modules
	@set -eu; \
	for module in $(GO_MODULE_DIRS); do \
		echo "==> $(1) ($$module)"; \
		(cd "$$module" && export GOWORK=off && $(2)); \
	done
endef

build:
	go build -v ./...

install-dependencies:
	$(call run-in-go-modules,download dependencies,go mod download)

install: build
	go install -v ./...

fmt: fmt-go fmt-terraform

fmt-go:
	gofmt -s -w -e .

fmt-terraform:
	terraform fmt -recursive examples/ dev/local-devloop/

lint: lint-go lint-sh

lint-go:
	golangci-lint run

lint-sh:
	find . -path './.git' -prune -o -type f -name '*.sh' -exec shellcheck {} +

generate:
	cd tools; GOWORK=off go generate tools.go

test: test-unit test-acc

test-unit:
	go test -v -cover -timeout=120s -parallel=10 ./...

test-acc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

check-go-modules:
	$(call run-in-go-modules,check Go module,go mod download && packages="$$(go list ./...)" && (go test ./... || [ -z "$$packages" ]) && if [ -n "$$packages" ]; then go build ./...; fi)

release-snapshot:
	VERSION=$(RELEASE_SNAPSHOT_VERSION) ./scripts/build-release-artifacts.sh

.PHONY: fmt fmt-go fmt-terraform lint lint-go lint-sh test test-unit test-acc check-go-modules build install-dependencies install generate release-snapshot
