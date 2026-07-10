FROM mcr.microsoft.com/devcontainers/go:2.1-1.25-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        gnupg \
        make \
    && install -d -m 0755 /etc/apt/keyrings \
    && curl -fsSL https://apt.releases.hashicorp.com/gpg | gpg --dearmor -o /etc/apt/keyrings/hashicorp-archive-keyring.gpg \
    && chmod a+r /etc/apt/keyrings/hashicorp-archive-keyring.gpg \
    && . /etc/os-release \
    && echo "deb [signed-by=/etc/apt/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com ${VERSION_CODENAME} main" > /etc/apt/sources.list.d/hashicorp.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends terraform \
    && rm -rf /var/lib/apt/lists/*

RUN GOPATH=/tmp/gopath \
    GOMODCACHE=/tmp/gopath/pkg/mod \
    GOCACHE=/tmp/go-build-cache \
    GOBIN=/usr/local/bin \
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest \
    && rm -rf /tmp/gopath /tmp/go-build-cache \
    && install -d -o vscode -g golang -m 2775 /go/pkg /go/pkg/mod \
    && chmod -R g+rwX /go
