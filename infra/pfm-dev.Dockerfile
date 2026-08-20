# pfm-dev — the ONE dev+e2e image: a fresh machine per run for building and
# testing pfm inside the fence (docs/dev/isolated-dev-foundation.md).
# The worktree is edited on the HOST; this container only builds and tests it.
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl git zsh tmux \
 && rm -rf /var/lib/apt/lists/*
# Go pinned to pfm/go.mod — bump both together or the fence tests a different compiler.
ARG GO_VERSION=1.24.13
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz
ENV HOME=/root \
    PATH=/usr/local/go/bin:/root/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    CGO_ENABLED=0
WORKDIR /work
