FROM mcr.microsoft.com/devcontainers/go:1.25-bookworm

ARG TERRAFORM_DOCS_VERSION=0.20.0

RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends \
        apt-transport-https \
        ca-certificates \
        curl \
        gnupg \
        lsb-release \
        make \
        pre-commit \
        python3 \
        ripgrep \
        shellcheck \
        unzip \
        wget \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN wget -O- https://apt.releases.hashicorp.com/gpg | gpg --dearmor > /usr/share/keyrings/hashicorp-archive-keyring.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" > /etc/apt/sources.list.d/hashicorp.list \
    && apt-get update \
    && apt-get -y install --no-install-recommends terraform \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN arch="$(dpkg --print-architecture)" \
    && case "$arch" in \
        amd64) terraform_docs_arch="amd64" ;; \
        arm64) terraform_docs_arch="arm64" ;; \
        *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; \
    esac \
    && curl -sSLo /tmp/terraform-docs.tar.gz "https://terraform-docs.io/dl/v${TERRAFORM_DOCS_VERSION}/terraform-docs-v${TERRAFORM_DOCS_VERSION}-Linux-${terraform_docs_arch}.tar.gz" \
    && tar -xzf /tmp/terraform-docs.tar.gz -C /tmp terraform-docs \
    && chmod +x /tmp/terraform-docs \
    && mv /tmp/terraform-docs /usr/local/bin/terraform-docs \
    && rm /tmp/terraform-docs.tar.gz

RUN go version \
    && terraform --version \
    && terraform-docs --version
