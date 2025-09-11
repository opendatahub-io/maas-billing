# MaaS Billing Makefile

# Container Engine to be used for building image and with kind
CONTAINER_ENGINE ?= docker
ifeq (podman,$(CONTAINER_ENGINE))
    CONTAINER_ENGINE_EXTRA_FLAGS ?= --load
endif

# Image settings
REPO ?= ghcr.io/your-org/maas-maas-api
TAG ?= latest
FULL_IMAGE ?= $(REPO):$(TAG)

PROJECT_DIR:=$(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))

BINARY_NAME := maas-api
BUILD_DIR := $(PROJECT_DIR)/bin

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
	@echo "  make build-image REPO=ghcr.io/myorg/maas-api TAG=v1.0.0"
	@echo "  make push-image REPO=ghcr.io/myorg/maas-api TAG=latest"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: fmt
fmt: ## Format Go code using gofmt
	@echo "Formatting Go code..."
	gofmt -s -w .
	@echo "Code formatting complete"

.PHONY: fmt-check
fmt-check: ## Check if Go code is formatted
	@echo "Checking Go code formatting..."
	if [ -n "$$(gofmt -l .)" ]; then \
		echo "The following files need formatting:"; \
		gofmt -l .; \
		exit 1; \
	fi
	@echo "All Go code is properly formatted"

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

.PHONY: lint
lint: fmt-check vet ## Run all linting checks
	@echo "All linting checks passed"

.PHONY: tidy
tidy: ## Tidy Go modules
	@echo "Tidying Go modules..."
	go mod tidy
	go mod verify

.PHONY: deps
deps: ## Download Go dependencies
	@echo "Downloading Go dependencies..."
	go mod download

.PHONY: build
build: deps fmt test ## Build the maas-api binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

SRC_DIRS := $(PROJECT_DIR)/cmd $(PROJECT_DIR)/internal
PKGS := $(foreach d,$(SRC_DIRS),$(d)/...)
TEST_FLAGS ?= -race -coverprofile=coverage.out
.PHONY: test
test: ## Run Go tests
	@echo "Running tests in..."
	go test -v $(TEST_FLAGS) $(PKGS)
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Test coverage report generated: $(abspath coverage.html)"

.PHONY: build-image
build-image: ## Build container image (use REPO= and TAG= to specify image)
	@if [ -z "$(REPO)" ]; then \
		echo "Error: REPO is required. Usage: make build-image REPO=ghcr.io/myorg/maas-api"; \
		exit 1; \
	fi
	@echo "Building container image $(FULL_IMAGE)..."
	$(CONTAINER_ENGINE) build $(CONTAINER_ENGINE_EXTRA_FLAGS) -t $(FULL_IMAGE) .
	@echo "Container image $(FULL_IMAGE) built successfully"

.PHONY: push-image
push-image: ## Push container image (use REPO= and TAG= to specify image)
	@if [ -z "$(REPO)" ]; then \
		echo "Error: REPO is required. Usage: make push-image REPO=ghcr.io/myorg/maas-api"; \
		exit 1; \
	fi
	@echo "Pushing container image $(FULL_IMAGE)..."
	@$(CONTAINER_ENGINE) push $(FULL_IMAGE)
	@echo "Container image $(FULL_IMAGE) pushed successfully"

.PHONY: build-push-image
build-push-image: build-image push-image ## Build and push container image

.PHONY: deploy-dev
deploy-dev: ## Deploy to development
	kubectl create namespace maas-api || true
	cd $(PROJECT_DIR)/deploy/overlays/dev && \
		cp kustomization.yaml kustomization.yaml.backup && \
		trap 'mv kustomization.yaml.backup kustomization.yaml 2>/dev/null || true' EXIT && \
		kustomize edit set image maas-api=$(FULL_IMAGE) && \
		kustomize build . | kubectl apply -f -

.PHONY: run
run: build ## Run the maas-api locally
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

