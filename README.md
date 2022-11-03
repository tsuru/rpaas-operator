# RPaaS v2

[![CI](https://github.com/tsuru/rpaas-operator/actions/workflows/ci.yaml/badge.svg)](https://github.com/tsuru/rpaas-operator/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/tsuru/rpaas-operator)](https://goreportcard.com/report/github.com/tsuru/rpaas-operator)

## What is it?

RPaaS v2 (short for Reverse Proxy as a Service v2) is a system that allows managing fully configurable Nginx web servers as simple Tsuru service instances. With it, developers can expose their apps on Tsuru to the wild using the old fashion Tsuru API and tooling.

Powered by Nginx OSS, developers can use several features such as:
* Advanced L7 routing;
* Cache;
* Canary deployments of Tsuru apps;
* Web Application Firewall - WAF;
* Rate limit / DDoS protection;
* Golden signals;
* GeoIP;
* TLS termination (w/ session resumption);
* and so on.

**NOTE**: As you may realize this project it's pretty similar to Nginx Ingress but each RPaaS instance provides an fully independent Nginx web server.

## Getting started

This project consists of several parts, namely:

1. RPaaS v2 API: web API which implements the Tsuru service contract.
2. RPaaS v2 purger API: web API which allows purging the cached resources in  live Nginx pods;
3. RPaaS v2 plugin: command line tool to manage RPaaS instances (compatible with Tsuru plugin - aka `tsuru rpaasv2`);
4. Kubernetes operator: CRD and controller which allows managing RPaaS instances using the Kubernetes API.

### Running locally

You will need a local Kubernetes cluster. There are bunch of tools to provision it, we recommend [kind](https://kind.sigs.k8s.io/).

**Warning**: Keep in mind that all following steps are going to use the Kubernetes cluster from your current `KUBECONFIG` config file (which defaults to `~/.kube/config`).

After cluster created, you need to install the [Nginx Operator](https://github.com/tsuru/nginx-operator) and [Cert Manager](https://github.com/cert-manager/cert-manager) with [Helm](https://helm.sh/).

```bash
# installing Nginx Operator
helm repo add tsuru https://tsuru.github.io/charts
helm install --create-namespace --namespace nginx-operator nginx-operator tsuru/nginx-operator

# installing Cert Manager
helm repo add jetstack https://charts.jetstack.io
helm install --create-namespace --namespace cert-manager --set installCRDs=true cert-manager jetstack/cert-manager
```

In a terminal session, run the following commands to start the Kubernetes operator subsystem.
```bash
# generate and apply all CRDs to Kubernetes
make deploy

# running the controller
make run
```

In another terminal session, run the following command to start the RPaaS v2 API.
```bash
# starts the web API on :9999/TCP
make run/api
```

### Testing

After following the steps above you can create and manage the instance from RPaaS v2 plugin:

```bash
# creates a RPaaS instance
kubectl apply -n rpaasv2 -f test/testdata/rpaas-full.yaml

# builds and uses the RPaaS v2 plugin to fetch infos about the instance created above
make build/plugin/rpaasv2
./bin/rpaasv2 --rpaas-url http://localhost:9999 info -s rpaasv2 -i my-instance
```

## Installing tsuru plugin

If you have the [tsuru-client](https://github.com/tsuru/tsuru-client/) installed, it is as easy as running:

```bash
tsuru plugin install rpaasv2 "https://github.com/tsuru/rpaas-operator/releases/latest/download/manifest.json"
```
