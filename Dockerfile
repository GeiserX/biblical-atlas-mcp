# Both bases are pinned by digest; dependabot moves the digests. The builder
# is go1.27.4 exactly, the toolchain go.mod asks for: the golang image sets
# GOTOOLCHAIN=local, so a floating 1.27 tag would not enforce it.
FROM --platform=$BUILDPLATFORM golang:1.27.4@sha256:08bcb99a38f85a265f9a93d1d3323e2440eec333380f3ea26ee0dec9264bb41c AS builder
ARG TARGETOS TARGETARCH VERSION=dev COMMIT=none DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags "-s -w -X github.com/geiserx/biblical-atlas-mcp/version.Version=${VERSION} -X github.com/geiserx/biblical-atlas-mcp/version.Commit=${COMMIT} -X github.com/geiserx/biblical-atlas-mcp/version.Date=${DATE}" \
    -o /out/biblical-atlas-mcp ./cmd/server

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
# The base image can lag the CA bundle; take the current one, which drops
# the compromised eMudhra (emSign) intermediates.
RUN apk upgrade --no-cache ca-certificates-bundle
LABEL io.modelcontextprotocol.server.name="io.github.GeiserX/biblical-atlas-mcp"
COPY --from=builder /out/biblical-atlas-mcp /usr/local/bin/biblical-atlas-mcp
USER 65534:65534
EXPOSE 8080
# Listening on every interface needs MCP_AUTH_TOKEN; the server refuses to
# start without it. Pass it with docker run -e MCP_AUTH_TOKEN=...
ENV LISTEN_ADDR=0.0.0.0:8080
ENV BIBLICAL_ATLAS_CACHE_DIR=/tmp/biblical-atlas-mcp
ENTRYPOINT ["/usr/local/bin/biblical-atlas-mcp"]
