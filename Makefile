TAG=latest
IMAGE_OPERATOR=tsuru/rpaas-operator
IMAGE_API=tsuru/rpaas-api
KUBECONFIG ?= ~/.kube/config

git_tag    := $(shell git describe --tags --abbrev=0 2>/dev/null || echo 'untagged')
git_commit := $(shell git rev-parse HEAD 2>/dev/null | cut -c1-7)
go_root    := $(shell go env GOROOT)

RPAAS_OPERATOR_VERSION ?= $(git_tag)/$(git_commit)
GO_LDFLAGS ?= -X=github.com/tsuru/rpaas-operator/version.Version=$(RPAAS_OPERATOR_VERSION)

.PHONY: test test/all test/integration deploy deploy/crds local build push build-api api build/plugin/rpaasv2

test/all: test test/integration

test:
	go test -race ./... -cover

test/integration:
	./scripts/localkube-integration.sh

lint:
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)
	go install ./...
	golangci-lint run --config ./.golangci.yml ./...

deploy:
	kubectl apply -R -f deploy/

deploy/crds:
	kubectl apply -f deploy/crds/

local: deploy/crds
	operator-sdk up local --go-ldflags $(GO_LDFLAGS)

generate:
	GOROOT=$(go_root) operator-sdk generate k8s

build: build/plugin/rpaasv2
	operator-sdk build $(IMAGE_OPERATOR):$(TAG) --go-build-args "-ldflags $(GO_LDFLAGS)"
	docker build -t $(IMAGE_API):$(TAG) .

build/plugin/rpaasv2:
	@mkdir -p build/_output/bin/
	go build -ldflags $(GO_LDFLAGS) -o build/_output/bin/rpaasv2 ./cmd/plugin/rpaasv2

push: build
	docker push $(IMAGE_OPERATOR):$(TAG)
	docker push $(IMAGE_API):$(TAG)

build-api:
	CGO_ENABLED=0 go build -o rpaas-api ./cmd/api

api: deploy/crds
	go run ./cmd/api
