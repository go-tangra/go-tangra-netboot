# Makefile for Netboot Service

# app.mk lives at the go-tangra-portal root and supplies VERSION, GOFLAGS and
# LDFLAGS. Its location relative to this module depends on how the tree is
# checked out: nested as <portal>/app/<name>/service, or as a sibling of
# go-tangra-portal. Rather than hard-coding one depth, search the candidates in
# order and use the first that exists.
NETBOOT_MK_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
APP_MK ?= $(firstword $(wildcard \
	$(NETBOOT_MK_DIR)../../../app.mk \
	$(NETBOOT_MK_DIR)../../app.mk \
	$(NETBOOT_MK_DIR)../app.mk \
	$(NETBOOT_MK_DIR)../go-tangra-portal/app.mk \
	$(NETBOOT_MK_DIR)../../go-tangra-portal/app.mk))

ifeq ($(APP_MK),)
$(error could not locate app.mk; set APP_MK=/path/to/go-tangra-portal/app.mk)
endif

include $(APP_MK)

# app.mk derives VERSION from git tags or the portal's .env; a standalone
# checkout has neither, and an empty VERSION would render an invalid
# "image:" tag, so fall back to an explicit development version.
ifeq ($(strip $(VERSION)),)
VERSION := 0.0.0-dev
endif

# The Docker build context is this module's own directory: every COPY in the
# Dockerfile (go.mod, frontend/, configs/) is module-relative, so a wider
# context would not resolve them.
DOCKER_CONTEXT ?= .

NETBOOT_IMAGE_NAME ?= menta2l/netboot-service
NETBOOT_IMAGE_TAG ?= $(VERSION)
DOCKER_REGISTRY ?=

# Minimum acceptable statement coverage across the internal packages.
COVERAGE_THRESHOLD ?= 80

# Build the server binary
.PHONY: build-server
build-server:
	@echo "Building Netboot server..."
	@go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o ./bin/netboot-server ./cmd/server

# Build Docker image
.PHONY: docker
docker:
	@echo "Building Docker image $(NETBOOT_IMAGE_NAME):$(NETBOOT_IMAGE_TAG)..."
	@docker build \
		-t $(NETBOOT_IMAGE_NAME):$(NETBOOT_IMAGE_TAG) \
		-t $(NETBOOT_IMAGE_NAME):latest \
		--build-arg APP_VERSION=$(VERSION) \
		-f ./Dockerfile \
		$(DOCKER_CONTEXT)

.PHONY: docker-tag
docker-tag: docker
ifdef DOCKER_REGISTRY
	@echo "Tagging image for registry $(DOCKER_REGISTRY)..."
	@docker tag $(NETBOOT_IMAGE_NAME):$(NETBOOT_IMAGE_TAG) $(DOCKER_REGISTRY)/$(NETBOOT_IMAGE_NAME):$(NETBOOT_IMAGE_TAG)
	@docker tag $(NETBOOT_IMAGE_NAME):latest $(DOCKER_REGISTRY)/$(NETBOOT_IMAGE_NAME):latest
endif

.PHONY: docker-push
docker-push: docker-tag
ifdef DOCKER_REGISTRY
	@echo "Pushing image to $(DOCKER_REGISTRY)..."
	@docker push $(DOCKER_REGISTRY)/$(NETBOOT_IMAGE_NAME):$(NETBOOT_IMAGE_TAG)
	@docker push $(DOCKER_REGISTRY)/$(NETBOOT_IMAGE_NAME):latest
else
	@echo "Pushing image to Docker Hub..."
	@docker push $(NETBOOT_IMAGE_NAME):$(NETBOOT_IMAGE_TAG)
	@docker push $(NETBOOT_IMAGE_NAME):latest
endif

.PHONY: docker-buildx
docker-buildx:
	@echo "Building multi-platform Docker image..."
	@docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(NETBOOT_IMAGE_NAME):$(NETBOOT_IMAGE_TAG) \
		-t $(NETBOOT_IMAGE_NAME):latest \
		--build-arg APP_VERSION=$(VERSION) \
		-f ./Dockerfile \
		$(DOCKER_CONTEXT)

# Run the server locally
.PHONY: run-server
run-server:
	@go run ./cmd/server -c ./configs

# Regenerate protobuf stubs, the OpenAPI document and the descriptor set
.PHONY: proto
proto:
	@buf lint
	@buf generate
	@buf generate --template buf.openapi.gen.yaml
	@buf build -o cmd/server/assets/descriptor.bin

# Generate wire dependencies
.PHONY: wire
wire:
	@cd ./cmd/server && wire

.PHONY: generate
generate: proto wire
	@echo "Generation complete!"

# Run tests
.PHONY: test
test:
	@go test -race ./...

# Run tests with coverage across every internal package
.PHONY: test-cover
test-cover:
	@go test -race -coverpkg=./internal/... -coverprofile=coverage.out ./internal/...
	@go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out | tail -1
	@echo "Coverage report generated: coverage.html"

# Fail if coverage regresses below the threshold
.PHONY: test-cover-check
test-cover-check:
	@go test -coverpkg=./internal/... -coverprofile=coverage.out ./internal/... > /dev/null
	@total=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | tr -d '%'); \
	echo "total coverage: $$total% (threshold $(COVERAGE_THRESHOLD)%)"; \
	awk -v t="$$total" -v m=$(COVERAGE_THRESHOLD) 'BEGIN { exit (t+0 >= m+0) ? 0 : 1 }' || \
	{ echo "coverage below threshold"; exit 1; }

.PHONY: lint
lint:
	@gofmt -l . | grep -v '^gen/' | tee /dev/stderr | wc -l | grep -q '^0$$'
	@go vet ./...

.PHONY: clean
clean:
	@rm -rf ./bin
	@rm -f coverage.out coverage.html
	@echo "Clean complete!"
