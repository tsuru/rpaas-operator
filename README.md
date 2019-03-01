# RPaaS Operator

This project has two parts: operator and API

## Operator

The [operator](https://coreos.com/operators/) is responsible for managing RPaaS instances running inside the cluster.

### Running locally

```sh
make local
```

### Running in Kubernetes

- build the docker image: `make build`
- push the image to a registry accessible from your cluster: `docker push ...`
- `make deploy`

## API

The API doesn't need to run inside Kubernetes, but it needs credentials to access Kubernetes API, to manage RPaaS instances, binds and plans.

It follows [tsuru service API contract](https://app.swaggerhub.com/apis/tsuru/tsuru-service_api/1.0.0).

### Running locally

To run outside the cluster, it needs a `KUBECONFIG` env var, pointing to your Kubernetes configuration. Start the API with:

```sh
make api
```

### Running in Kubernetes

When running inside the cluster, the API knows how to get the required credentials. Follow these steps:

- build the docker image: `docker build . -t my-registry/tsuru/rpaas-api`
- push the image to a registry accessible from your cluster: `docker push my-registry/tsuru/rpaas-api`
- start with `kubectl apply -f deploy/api.yaml`
