# The official Go image tracks the latest Go 1.25 patch, so it is our toolchain source.
FROM golang:1.25-bookworm

COPY dev/go/go.version /tmp/go.version
RUN test "$(go env GOVERSION | sed 's/^go//')" = "$(awk '!/^[[:space:]]*#/ && NF { print; exit }' /tmp/go.version)" \
    && rm /tmp/go.version

USER root

ENV HOME=/root
ENV GOPATH=$HOME/go-packages
ENV GOROOT=/usr/local/go
ENV PATH=/usr/local/go/bin:$GOPATH/bin:$PATH

ARG TERRAFORM_VERSION=1.15.8-1
ARG GOLANGCI_LINT_VERSION=v2.12.2

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        gnupg \
        make \
        shellcheck \
    && install -d -m 0755 /etc/apt/keyrings \
    && curl -fsSL https://apt.releases.hashicorp.com/gpg | gpg --dearmor -o /etc/apt/keyrings/hashicorp-archive-keyring.gpg \
    && chmod a+r /etc/apt/keyrings/hashicorp-archive-keyring.gpg \
    && . /etc/os-release \
    && echo "deb [signed-by=/etc/apt/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com ${VERSION_CODENAME} main" > /etc/apt/sources.list.d/hashicorp.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends terraform=${TERRAFORM_VERSION} \
    && rm -rf /var/lib/apt/lists/*

RUN GOBIN=/usr/local/bin \
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION} \
    && rm -rf "$HOME/.cache" "$HOME/go-packages/pkg"
