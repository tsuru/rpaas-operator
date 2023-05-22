#!/usr/bin/env bash

# Copyright 2020 tsuru authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
set -eu -o pipefail

[[ -n ${DEBUG:-} ]] && set -x

readonly DOCKER=${DOCKER:-docker}
readonly HELM=${HELM:-helm}
readonly KIND=${KIND:-kind}
readonly KUBECTL=${KUBECTL:-kubectl}
readonly KUSTOMIZE=${KUSTOMIZE:-kustomize}
readonly MINIKUBE=${MINIKUBE:-minikube}

readonly CLUSTER_PROVIDER=${CLUSTER_PROVIDER:-kind}
readonly NAMESPACE=${NAMESPACE:-rpaasv2-system}

readonly INSTALL_CERT_MANAGER=${INSTALL_CERT_MANAGER:-}
readonly CHART_VERSION_CERT_MANAGER=${CHART_VERSION_CERT_MANAGER:-1.11.2}
readonly CHART_VERSION_RPAAS_OPERATOR=${CHART_VERSION_RPAAS_OPERATOR:-0.10.1}

function onerror() {
  echo
  echo "RPAAS OPERATOR LOGS:"
  ${KUBECTL} logs -n ${NAMESPACE} deploy/rpaas-operator || true
  echo
  echo "NGINX OPERATOR LOGS:"
  ${KUBECTL} logs -n ${NAMESPACE} deploy/rpaas-operator-nginx-operator || true
  echo
  echo "RPAAS API LOGS:"
  ${KUBECTL} logs -n ${NAMESPACE} deploy/rpaas-api|| true
  echo

  [[ -n ${kubectl_port_forward_pid} ]] && kill ${kubectl_port_forward_pid}
}

install_cert_manager() {
  [[ -z ${INSTALL_CERT_MANAGER} ]] && return

  ${HELM} repo add --force-update jetstack https://charts.jetstack.io

  ${HELM} upgrade --install --atomic \
    --namespace ${NAMESPACE} --version ${CHART_VERSION_CERT_MANAGER} \
    --set installCRDs=true \
    cert-manager jetstack/cert-manager
}

install_rpaas_operator() {
  ${HELM} repo add --force-update tsuru https://tsuru.github.io/charts

  ${HELM} upgrade --install --atomic \
    --namespace ${NAMESPACE} --version ${CHART_VERSION_RPAAS_OPERATOR} \
    --set image.repository=localhost/tsuru/rpaas-operator \
    --set image.tag=integration \
    --set image.pullPolicy=Never \
    rpaas-operator tsuru/rpaas-operator
}

install_rpaas_api() {
  (
    cd config/api
    ${KUSTOMIZE} edit set image tsuru/rpaas-api=localhost/tsuru/rpaas-api:integration
    ${KUSTOMIZE} edit set namespace rpaasv2-system
  )

  ${KUBECTL} apply -n ${NAMESPACE} -k config/api
}

build_rpaasv2_container_images() {
  ${DOCKER} build -t localhost/tsuru/rpaas-operator:integration -f Dockerfile.operator .
  ${DOCKER} build -t localhost/tsuru/rpaas-api:integration -f Dockerfile.api .

  case ${CLUSTER_PROVIDER} in
    minikube)
      ${DOCKER} save localhost/tsuru/rpaas-operator:integration | ${MINIKUBE} image load -
      ${DOCKER} save localhost/tsuru/rpaas-api:integration | ${MINIKUBE} image load -
      ;;

    kind)
      for image in "rpaas-operator" "rpaas-api"; do
        ${DOCKER} save "localhost/tsuru/${image}:integration" -o "${image}.tar"
        ${KIND} load image-archive "${image}.tar"
        rm "${image}.tar"
      done
      ;;

    *)
      print "Invalid local cluster provider (got ${CLUSTER_PROVIDER}, supported: kind, minikube)" >&2
      exit 1;;
  esac
}

main() {
  ${KUBECTL} cluster-info
  ${KUBECTL} get all

  ${KUBECTL} get namespace ${NAMESPACE} >/dev/null 2>&1 || \
    ${KUBECTL} create namespace ${NAMESPACE}

  build_rpaasv2_container_images

  install_cert_manager
  install_rpaas_operator
  install_rpaas_api

  sleep 5s

  trap onerror ERR

  local_rpaas_api_port=39999
  ${KUBECTL} -n ${NAMESPACE} port-forward svc/rpaas-api ${local_rpaas_api_port}:9999 --address=127.0.0.1 &
  kubectl_port_forward_pid=${!}

  sleep 5s

  make build/plugin/rpaasv2

  RPAAS_PLUGIN_BIN=$(pwd)/bin/rpaasv2                          \
  RPAAS_API_ADDRESS="http://127.0.0.1:${local_rpaas_api_port}" \
  RPAAS_OPERATOR_INTEGRATION=1                                 \
  go test -v ./test/...

  kill ${kubectl_port_forward_pid}
}

main $@
