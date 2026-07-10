FROM mcr.microsoft.com/devcontainers/go:2.1-1.25-bookworm

ENV HOME=/root
ENV GOPATH=$HOME/go-packages
ENV PATH=$GOPATH/bin:$PATH

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

RUN GOBIN=/usr/local/bin \
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest \
    && rm -rf "$HOME/.cache" "$HOME/go-packages/pkg"
