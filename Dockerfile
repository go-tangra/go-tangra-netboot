##################################
# Stage 0: Build frontend module
##################################

FROM node:20-alpine AS frontend-builder

RUN npm install -g pnpm@9

WORKDIR /frontend
COPY frontend/package.json frontend/pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile || pnpm install
COPY frontend/ .
RUN pnpm build

##################################
# Stage 1: Build Go executable
##################################

FROM golang:1.25-alpine AS builder

ARG APP_VERSION=1.0.0

# Enable toolchain auto-download for newer Go versions
ENV GOTOOLCHAIN=auto

RUN apk add --no-cache git make curl

# Install buf for proto descriptor generation
RUN curl -sSL "https://github.com/bufbuild/buf/releases/latest/download/buf-$(uname -s)-$(uname -m)" -o /usr/local/bin/buf && \
    chmod +x /usr/local/bin/buf

WORKDIR /src

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Regenerate the proto descriptor so the embedded copy is always current
RUN buf build -o cmd/server/assets/descriptor.bin

# Copy the federated frontend bundle in for go:embed
COPY --from=frontend-builder /frontend/dist cmd/server/assets/frontend-dist/

RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build -ldflags "-X main.version=${APP_VERSION} -s -w" \
    -o /src/bin/netboot-server \
    ./cmd/server

##################################
# Stage 2: Create runtime image
##################################

FROM alpine:3.20

ARG APP_VERSION=1.0.0

RUN apk --no-cache add ca-certificates tzdata

ENV TZ=UTC

WORKDIR /app

COPY --from=builder /src/bin/netboot-server /app/bin/netboot-server
COPY --from=builder /src/configs/ /app/configs/

# Run as a non-root user. The module needs no privileged ports: DHCP and TFTP
# are served by the remote netbootd host, not by this container.
RUN addgroup -g 1000 netboot && \
    adduser -D -u 1000 -G netboot netboot && \
    mkdir -p /app/certs && chown -R netboot:netboot /app

USER netboot:netboot

# 10000 gRPC, 10001 HTTP (frontend + descriptors), 10010 metrics
EXPOSE 10000 10001 10010

CMD ["/app/bin/netboot-server", "-c", "/app/configs"]

LABEL org.opencontainers.image.title="Netboot Service" \
      org.opencontainers.image.description="Tangra module federating a remote netbootd provisioning service" \
      org.opencontainers.image.version="${APP_VERSION}"
