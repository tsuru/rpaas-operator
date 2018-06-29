TAG=latest
IMAGE=tsuru/nginx-operator

.PHONY: test deploy local build push

test:
	go test ./...

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