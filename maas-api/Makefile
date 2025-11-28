# MaaS Billing Makefile

# Container Engine to be used for building image and with kind
CONTAINER_ENGINE ?= podman

# Image settings
REPO ?= quay.io/opendatahub/maas-api
TAG ?= latest
FULL_IMAGE ?= $(REPO):$(TAG)

PROJECT_DIR:=$(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))

BINARY_NAME := maas-api
BUILD_DIR := $(PROJECT_DIR)/bin
LOCALBIN := $(BUILD_DIR)/tools

# Go settings
GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GO_STRICTFIPS ?= false

ifeq ($(GO_STRICTFIPS),true)
  GOEXPERIMENT ?= strictfipsruntime
  CGO_ENABLED  ?= 1
endif

GO_ENV := GOOS=$(GOOS) GOARCH=$(GOARCH)
ifdef GOEXPERIMENT
  GO_ENV += GOEXPERIMENT=$(GOEXPERIMENT)
endif
ifdef CGO_ENABLED
  GO_ENV += CGO_ENABLED=$(CGO_ENABLED)
endif

# Git settings
GIT_COMMIT := $(shell git rev-parse --is-inside-work-tree >/dev/null 2>&1 && git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --is-inside-work-tree >/dev/null 2>&1 && git rev-parse --abbrev-ref HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Ldflags for build info
LDFLAGS ?= -ldflags "-s -w -X main.version=$(TAG) -X main.commit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)"

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

LINT_FIX ?= false
.PHONY: lint
lint: $(GOLANGCI_LINT) ## Executes golangci linters (use LINT_FIX=true to fix lint issues)
	$(GOLANGCI_LINT) fmt
	$(GOLANGCI_LINT) run $(if $(filter true,$(LINT_FIX)),--fix)

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
build: deps lint test binary ## Build the maas-api binary

.PHONY: binary
binary: $(BUILD_DIR)
	$(GO_ENV) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

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
	@echo "Building container image $(FULL_IMAGE)..."
	$(CONTAINER_ENGINE) build $(CONTAINER_ENGINE_EXTRA_FLAGS) -t $(FULL_IMAGE) .
	@echo "Container image $(FULL_IMAGE) built successfully"

.PHONY: push-image
push-image: ## Push container image (use REPO= and TAG= to specify image)
	@echo "Pushing container image $(FULL_IMAGE)..."
	@$(CONTAINER_ENGINE) push $(FULL_IMAGE)
	@echo "Container image $(FULL_IMAGE) pushed successfully"

.PHONY: build-push-image
build-push-image: build-image push-image ## Build and push container image

## Deployment

PRE_DEPLOY_STEP ?=
define deploy
	kubectl create namespace maas-api || true
	cd $(PROJECT_DIR)/deploy/$(1) && \
		set -eu; \
		cp kustomization.yaml kustomization.yaml.backup && \
		trap 'mv kustomization.yaml.backup kustomization.yaml 2>/dev/null || true' EXIT INT TERM && \
		kustomize edit set image maas-api=$(FULL_IMAGE) && \
		$(if $(PRE_DEPLOY_STEP),$(PRE_DEPLOY_STEP) &&) \
		kustomize build . --load-restrictor LoadRestrictionsNone | kubectl apply -f -
endef

.PHONY: deploy-dev
deploy-dev: ## Deploy development version
	$(call deploy,overlays/dev)

default_run_flags := --debug
RUN_FLAGS ?= 
.PHONY: run
run: build ## Run the maas-api locally
	$(BUILD_DIR)/$(BINARY_NAME) $(default_run_flags) $(RUN_FLAGS) 

.PHONY: version
version: ## Show version information
	@echo "Image: $(FULL_IMAGE)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Git Branch: $(GIT_BRANCH)"
	@echo "Build Time: $(BUILD_TIME)"

# Default target
.DEFAULT_GOAL := help

## Tools
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.6.2

$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef