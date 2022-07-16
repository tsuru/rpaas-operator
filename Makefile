# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GO_BUILD_DIR ?= ./bin

all: test build

# Run tests
.PHONY: test
test: fmt vet lint
	go test -race -coverprofile cover.out ./...

.PHONY: lint
lint: golangci-lint
	$(GOLANGCI_LINT) run ./...

.PHONY: build
build: build/api build/manager build/plugin/rpaasv2 build/purger

.PHONY: build/api
build/api: build-dirs
	go build -o $(GO_BUILD_DIR)/ ./cmd/api

.PHONY: build/manager
build/manager: manager
	
.PHONY: build/plugin/rpaasv2
build/plugin/rpaasv2: build-dirs
	go build -o $(GO_BUILD_DIR)/ ./cmd/plugin/rpaasv2

.PHONY: build/purger
build/purger: build-dirs
	go build -o $(GO_BUILD_DIR)/ ./cmd/purger

.PHONY: build-dirs
build-dirs:
	@mkdir -p $(GO_BUILD_DIR)

# Build manager binary
.PHONY: manager
manager: generate fmt vet build-dirs
	go build -o $(GO_BUILD_DIR)/manager ./main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate fmt vet manifests
	go run main.go --health-probe-bind-address=0 --metrics-bind-address=0 --leader-elect=false

.PHONY: run/api
run/api:
	go run ./cmd/api

# Install CRDs into a cluster
.PHONY: install
install: manifests kustomize
	$(KUBECTL) apply -k config/crd

# Uninstall CRDs from a cluster
.PHONY: uninstall
uninstall: manifests kustomize
	$(KUBECTL) delete -k config/crd

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUBECTL) apply -k config/default

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) crd rbac:roleName=manager-role paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# Generate code
.PHONY: generate-cli
generate-cli: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: generate
generate: generate-cli manifests

# find or download controller-gen
# download controller-gen if necessary
.PHONY: controller-gen
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0 ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

.PHONY: kustomize
kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# find or download golangci-lint
# download golangci-lint if necessary
.PHONY: golangci-lint
golangci-lint:
ifeq (, $(shell which golangci-lint))
	@{ \
	set -e ;\
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) ;\
	}
GOLANGCI_LINT=$(GOBIN)/golangci-lint
else
GOLANGCI_LINT=$(shell which golangci-lint)
endif