#!/bin/bash

set -euo pipefail




get_os() {
  local os="$(uname -s)"
  case ${os} in
    Linux)
      echo -n "linux";;
    Darwin)
      echo -n "darwin";;
    *)
      echo "Unsupported operating system. (supported: Linux or Darwin)"
      exit 1;;
  esac
}

get_arch() {
  local arch="$(uname -m)"
  case ${arch} in
    amd64|x86_64)
      echo -n "amd64";;
    *)
      echo "Unsupported machine architecture. (supported: x86_64 and amd64)"
      exit 1;;
  esac
}

function onerror() {
  echo
  echo "RPAAS OPERATOR LOGS:"
  kubectl logs deploy/rpaas-operator-controller-manager -n ${rpaas_system_namespace} || true
  echo
  echo "NGINX OPERATOR LOGS:"
  kubectl logs deploy/nginx-operator-controller -n ${rpaas_system_namespace} || true
  echo
  echo "RPAAS API LOGS:"
  kubectl logs deploy/rpaas-api -n ${rpaas_system_namespace} || true
  echo

  [[ -n ${kubectl_port_forward_pid} ]] && kill ${kubectl_port_forward_pid}
}

trap onerror ERR

run_nginx_operator() {
  local namespace="${1}"
  local nginx_operator_dir="${GOPATH}/src/github.com/tsuru/nginx-operator"
  local nginx_operator_revision=$(go mod download -json github.com/tsuru/nginx-operator | jq .Version -r | awk -F '-' '{print $NF}')
  local tag=$(echo ${nginx_operator_revision} | tr v '\0')

  if [[ ! -d ${nginx_operator_dir} ]]; then
    mkdir -p $(dirname ${nginx_operator_dir})
    git clone https://github.com/tsuru/nginx-operator.git ${nginx_operator_dir}
  fi
  pushd ${nginx_operator_dir}
  git fetch --all
  git checkout ${nginx_operator_revision}
  popd

  echo "Pulling image of NGINX operator using tag \"${tag}\"..."
  docker pull tsuru/nginx-operator:${tag}

  kind load docker-image tsuru/nginx-operator:${tag}

  (cd ${nginx_operator_dir}/config/default && kustomize edit set namespace ${namespace})
  (cd ${nginx_operator_dir}/config/default && kustomize edit set image tsuru/nginx-operator=tsuru/nginx-operator:${tag})

  kustomize build ${nginx_operator_dir}/config/default | kubectl -n ${namespace} apply -f -

  kubectl rollout status deployment/nginx-operator-controller -n ${namespace}
}

run_rpaas_operator() {
  local namespace="${1}"
  local tag="${2:-"integration"}"

  echo "Building container images of RPaaS operator and API using tag \"${tag}\"..."
  docker build -t "tsuru/rpaas-operator:${tag}" -f Dockerfile.operator .
  docker build -t "tsuru/rpaas-api:${tag}" -f Dockerfile.api .

  echo tsuru/rpaas-{api,operator}:${tag} | tr ' ' '\n' |
  xargs -I{} kind load docker-image {}

  (cd ./config/default && kustomize edit set namespace ${namespace})
  (cd ./config/default && kustomize edit set image tsuru/rpaas-operator=tsuru/rpaas-operator:${tag})
  (cd ./config/default && kustomize edit set image tsuru/rpaas-api=tsuru/rpaas-api:${tag})

  kustomize build ./config/default | kubectl -n ${namespace} apply -f -

  kubectl rollout status deployment/rpaas-api -n ${namespace}
  kubectl rollout status deployment/rpaas-operator-controller-manager -n ${namespace}
}

[[ -n ${DEBUG:-} ]] && set -x


export GO111MODULE=on

# show some info about Kubernetes cluster
kubectl cluster-info
kubectl get all

rpaas_system_namespace="rpaas-system"

echo "Using namespace \"${rpaas_system_namespace}\" to run \"nginx-operator\" and \"rpaas-operator\"..."
kubectl delete namespace "${rpaas_system_namespace}" || true
kubectl create namespace "${rpaas_system_namespace}"

run_nginx_operator "${rpaas_system_namespace}" 
run_rpaas_operator "${rpaas_system_namespace}"

sleep 30s

kubectl get deployment --all-namespaces
kubectl get pods --all-namespaces

local_rpaas_api_port=39999

kubectl -n "${rpaas_system_namespace}" port-forward svc/rpaas-api ${local_rpaas_api_port}:9999 --address=127.0.0.1 &
kubectl_port_forward_pid=${!}

sleep 10s

make build/plugin/rpaasv2

RPAAS_PLUGIN_BIN=$(pwd)/build/_output/bin/rpaasv2            \
RPAAS_API_ADDRESS="http://127.0.0.1:${local_rpaas_api_port}" \
RPAAS_OPERATOR_INTEGRATION=1                                 \
go test -test.v ./test/...

kill ${kubectl_port_forward_pid}