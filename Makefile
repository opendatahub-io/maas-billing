# MaaS Billing Makefile

# Container Engine to be used for building image and with kind
CONTAINER_ENGINE ?= docker
ifeq (podman,$(CONTAINER_ENGINE))
    CONTAINER_ENGINE_EXTRA_FLAGS ?= --load
endif

# Image settings
REPO ?= ghcr.io/your-org/maas-key-manager
TAG ?= latest
FULL_IMAGE ?= $(REPO):$(TAG)

# Key Manager settings
KEY_MANAGER_DIR := deployment/kuadrant-openshift/key-manager
BINARY_NAME := key-manager
BUILD_DIR := ./bin
CMD_DIR := $(KEY_MANAGER_DIR)/cmd/key-manager

# Go settings
GO_VERSION := 1.24.2
GOOS ?= linux
GOARCH ?= amd64
CGO_ENABLED ?= 0

# Git settings
GIT_COMMIT := $(shell git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Ldflags for build info
LDFLAGS := -ldflags "-s -w -X main.version=$(TAG) -X main.commit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)"

.PHONY: help
help: ## Show this help message
	@echo "MaaS Billing Makefile"
	@echo ""
	@echo "Usage: make <target> [REPO=your-repo] [TAG=your-tag]"
	@echo ""
	@echo "Examples:"
	@echo "  make build-image REPO=ghcr.io/myorg/key-manager TAG=v1.0.0"
	@echo "  make push-image REPO=ghcr.io/myorg/key-manager TAG=latest"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: fmt
fmt: ## Format Go code using gofmt
	@echo "Formatting Go code..."
	@cd $(KEY_MANAGER_DIR) && gofmt -s -w .
	@echo "Code formatting complete"

.PHONY: fmt-check
fmt-check: ## Check if Go code is formatted
	@echo "Checking Go code formatting..."
	@cd $(KEY_MANAGER_DIR) && if [ -n "$$(gofmt -l .)" ]; then \
		echo "The following files need formatting:"; \
		gofmt -l .; \
		exit 1; \
	fi
	@echo "All Go code is properly formatted"

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	@cd $(KEY_MANAGER_DIR) && go vet ./...

.PHONY: lint
lint: fmt-check vet ## Run all linting checks
	@echo "All linting checks passed"

.PHONY: tidy
tidy: ## Tidy Go modules
	@echo "Tidying Go modules..."
	@cd $(KEY_MANAGER_DIR) && go mod tidy
	@cd $(KEY_MANAGER_DIR) && go mod verify

.PHONY: deps
deps: ## Download Go dependencies
	@echo "Downloading Go dependencies..."
	@cd $(KEY_MANAGER_DIR) && go mod download

.PHONY: build
build: fmt deps ## Build the key-manager binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@cd $(KEY_MANAGER_DIR) && CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o ../../$(BUILD_DIR)/$(BINARY_NAME) ./cmd/key-manager
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: test
test: ## Run Go tests
	@echo "Running tests..."
	@cd $(KEY_MANAGER_DIR) && go test -v -race -coverprofile=coverage.out ./...
	@cd $(KEY_MANAGER_DIR) && go tool cover -html=coverage.out -o coverage.html
	@echo "Test coverage report generated: $(KEY_MANAGER_DIR)/coverage.html"

.PHONY: test-short
test-short: ## Run Go tests (short mode)
	@echo "Running tests (short mode)..."
	@cd $(KEY_MANAGER_DIR) && go test -short -v ./...

.PHONY: build-image
build-image: ## Build container image (use REPO= and TAG= to specify image)
	@if [ -z "$(REPO)" ]; then \
		echo "Error: REPO is required. Usage: make build-image REPO=ghcr.io/myorg/key-manager"; \
		exit 1; \
	fi
	@echo "Building container image $(FULL_IMAGE)..."
	@cd $(KEY_MANAGER_DIR) && $(CONTAINER_ENGINE) build $(CONTAINER_ENGINE_EXTRA_FLAGS) -t $(FULL_IMAGE) .
	@echo "Container image $(FULL_IMAGE) built successfully"

.PHONY: push-image
push-image: ## Push container image (use REPO= and TAG= to specify image)
	@if [ -z "$(REPO)" ]; then \
		echo "Error: REPO is required. Usage: make push-image REPO=ghcr.io/myorg/key-manager"; \
		exit 1; \
	fi
	@echo "Pushing container image $(FULL_IMAGE)..."
	@$(CONTAINER_ENGINE) push $(FULL_IMAGE)
	@echo "Container image $(FULL_IMAGE) pushed successfully"

.PHONY: build-push-image
build-push-image: build-image push-image ## Build and push container image

.PHONY: run
run: build ## Run the key-manager locally
	@echo "Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

.PHONY: version
version: ## Show version information
	@echo "Image: $(FULL_IMAGE)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Git Branch: $(GIT_BRANCH)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(GO_VERSION)"

# Default target
.DEFAULT_GOAL := help

