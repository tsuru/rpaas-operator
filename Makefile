TAG=latest
IMAGE_OPERATOR=tsuru/rpaas-operator
IMAGE_API=tsuru/rpaas-api
KUBECONFIG=~/.kube/config

.PHONY: test test/all test/integration deploy local build push build-api api

test/all: test test/integration

test:
	go test -v ./...

test/integration:
	./scripts/localkube-integration.sh

deploy:
	kubectl apply -f deploy/

local: deploy
	operator-sdk up local

generate:
	operator-sdk generate k8s

build:
	operator-sdk build $(IMAGE_OPERATOR):$(TAG)
	docker build -t $(IMAGE_API):$(TAG) .

push: build
	docker push $(IMAGE_OPERATOR):$(TAG)
	docker push $(IMAGE_API):$(TAG)

build-api:
	CGO_ENABLED=0 go build -o rpaas-api ./cmd/api

api:
	cd cmd/api && KUBECONFIG=$(KUBECONFIG) go run main.go
