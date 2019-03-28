TAG=latest
IMAGE=tsuru/rpaas-operator
KUBECONFIG=~/.kube/config

.PHONY: test deploy local build push build-api api

test:
	go test -v ./...

deploy:
	kubectl apply -f deploy/

local: deploy
	operator-sdk up local

generate:
	operator-sdk generate k8s

build:
	operator-sdk build $(IMAGE):$(TAG)

push: build
	docker push $(IMAGE):$(TAG)

build-api:
	CGO_ENABLED=0 go build -o rpaas-api ./cmd/api

api:
	cd cmd/api && KUBECONFIG=$(KUBECONFIG) go run main.go
