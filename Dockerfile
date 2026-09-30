FROM --platform=$BUILDPLATFORM golang:1.27 AS builder
ARG TARGETOS TARGETARCH VERSION=dev COMMIT=none DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags "-s -w -X github.com/geiserx/biblical-atlas-mcp/version.Version=${VERSION} -X github.com/geiserx/biblical-atlas-mcp/version.Commit=${COMMIT} -X github.com/geiserx/biblical-atlas-mcp/version.Date=${DATE}" \
    -o /out/biblical-atlas-mcp ./cmd/server

FROM alpine:3.24
LABEL io.modelcontextprotocol.server.name="io.github.GeiserX/biblical-atlas-mcp"
COPY --from=builder /out/biblical-atlas-mcp /usr/local/bin/biblical-atlas-mcp
USER 65534:65534
EXPOSE 8080
ENV LISTEN_ADDR=0.0.0.0:8080
ENV BIBLICAL_ATLAS_CACHE_DIR=/tmp/biblical-atlas-mcp
ENTRYPOINT ["/usr/local/bin/biblical-atlas-mcp"]
