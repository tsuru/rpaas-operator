#!/bin/bash

set -euo pipefail

download_operator_sdk() {
  local version="${1}"
  local destination="${2}"

  local operator_sdk_bin="${destination}/operator-sdk"

  [[ -f "${operator_sdk_bin}" ]] && [[ $("${operator_sdk_bin}" version) == *"${version}"* ]] && echo -n "${operator_sdk_bin}" && return

  local os="$(get_os)"
  case ${os} in
    linux)
      os="linux-gnu";;
    darwin)
      os="apple-darwin";;
  esac

  curl -sLo ${operator_sdk_bin} "https://github.com/operator-framework/operator-sdk/releases/download/${version}/operator-sdk-${version}-x86_64-${os}"
  chmod +x ${operator_sdk_bin}
  echo -n ${operator_sdk_bin}  
}

download_kubectl() {
  local version="${1}"
  local destination="${2}"

  local kubectl_bin="${destination}/kubectl"

  [[ -f "${kubectl_bin}" ]] && [[ $("${kubectl_bin}" version --client) == *"${version}"* ]] && echo -n "${kubectl_bin}" && return

  local os="$(get_os)"
  local arch="$(get_arch)"

  curl -sLo ${kubectl_bin} "https://storage.googleapis.com/kubernetes-release/release/${version}/bin/${os}/${arch}/kubectl"
  chmod +x ${kubectl_bin}
  echo -n ${kubectl_bin}
}

download_kind() {
  local version="${1}"
  local destination="${2}"

  local kind_bin="${destination}/kind"

  [[ -f "${kind_bin}" ]] && [[ $(${kind_bin} version) == *"${version}"* ]] && echo -n "${kind_bin}" && return

  local os="$(get_os)"
  local arch="$(get_arch)"

  curl -sLo "${kind_bin}" "https://github.com/kubernetes-sigs/kind/releases/download/${version}/kind-${os}-${arch}"
  chmod +x "${kind_bin}"
  echo -n "${kind_bin}"
}

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

create_k8s_cluster() {
  local kind_bin="${1}"
  local name="${2}"
  local wait="${3:-"10m"}"

  clusters=$(kind get clusters)

  if [[ "${clusters}" == *"${cluster_name}"* ]]; then

    return
  fi

  ${kind_bin} create cluster \
    --name "${cluster_name}" --wait "10m" \
    --image "kindest/node:${KUBERNETES_VERSION}"
}

delete_k8s_cluster() {
  local kind_bin="${1}"
  local name=${2}

  ${kind_bin} delete cluster --name "${name}"
}

run_nginx_operator() {
  local kubectl_bin="${1}"
  local namespace="${2}"
  local kind_bin="${3}"
  local cluster_name="${4}"
  local tag="${5:-"integration"}"
  local nginx_operator_dir="${GOPATH}/src/github.com/tsuru/nginx-operator"
  local nginx_operator_revision=$(go mod download -json github.com/tsuru/nginx-operator | jq .Version -r | awk -F '-' '{print $NF}')

  if [[ ! -d ${nginx_operator_dir} ]]; then
    mkdir -p $(dirname ${nginx_operator_dir})
    git clone https://github.com/tsuru/nginx-operator.git ${nginx_operator_dir}
  fi
  pushd ${nginx_operator_dir}
  git fetch --all
  git checkout ${nginx_operator_revision}
  popd

  echo "Building container image of NGINX operator using tag \"${tag}\"..."
  make -C ${nginx_operator_dir} build TAG="${tag}"

  ${kind_bin} load docker-image --name ${cluster_name} tsuru/nginx-operator:${tag}

  ls ${nginx_operator_dir}/deploy/crds/*_crd.yaml |
  xargs -I{} ${kubectl_bin} -n ${namespace} apply -f {}

  ls ${nginx_operator_dir}/deploy/{role,service_account}.yaml |
  xargs -I{} ${kubectl_bin} -n ${namespace} apply -f {}

  sed -E "s/(namespace:) (.+)/\1 ${namespace}/" ${nginx_operator_dir}/deploy/role_binding.yaml |
  ${kubectl_bin} -n ${namespace} apply -f -

  sed -e 's/imagePullPolicy: Always/imagePullPolicy: Never/' ${nginx_operator_dir}/deploy/operator.yaml |
  sed -E "s|(tsuru/nginx-operator):latest|\1:${tag}|" |
  ${kubectl_bin} -n ${namespace} apply -f -
}

run_rpaas_operator() {
  local kubectl_bin="${1}"
  local namespace="${2}"
  local kind_bin="${3}"
  local cluster_name="${4}"
  local tag="${5:-"integration"}"

  echo "Building container images of RPaaS operator and API using tag \"${tag}\"..."
  make build TAG="${tag}"

  echo tsuru/rpaas-{api,operator}:${tag} | tr ' ' '\n' |
  xargs -I{} ${kind_bin} load docker-image --name ${cluster_name} {}

  ls ./deploy | grep -E ".+\.yaml" | grep -vE "api|operator" |
  xargs -I{} ${kubectl_bin} -n ${namespace} apply -f ./deploy/{}

  sed -E "s/(namespace:) (.+)/\1 ${namespace}/" ./deploy/role_binding.yaml |
  ${kubectl_bin} -n ${namespace} apply -f -

  sed -e 's/imagePullPolicy: Always/imagePullPolicy: Never/' deploy/operator.yaml |
  sed -e "s|tsuru/rpaas-operator|&:${tag}|" |
  ${kubectl_bin} -n ${namespace} apply -f -

  sed -e 's/imagePullPolicy: Always/imagePullPolicy: Never/' deploy/api.yaml |
  sed -e "s|tsuru/rpaas-api|&:${tag}|" |
  ${kubectl_bin} -n ${namespace} apply -f -

  ls ./deploy/crds/*_crd.yaml |
  xargs -I{} ${kubectl_bin} -n ${namespace} apply -f {}
}

function onerror() {
  echo
  echo "RPAAS OPERATOR LOGS:"
  ${kubectl_bin} logs deploy/rpaas-operator -n ${rpaas_system_namespace}
  echo
  echo "NGINX OPERATOR LOGS:"
  ${kubectl_bin} logs deploy/nginx-operator -n ${rpaas_system_namespace}
  echo
  echo "RPAAS API LOGS:"
  ${kubectl_bin} logs deploy/rpaas-api -n ${rpaas_system_namespace}
  echo

  [[ -n ${kubectl_port_forward_pid} ]] && kill ${kubectl_port_forward_pid}
}

trap onerror ERR

[[ -n ${DEBUG:-} ]] && set -x

# When KUBERNETES_VERSION isn't defined, use default
export KUBERNETES_VERSION="${KUBERNETES_VERSION:-"v1.14.3"}"
kind_version="v0.6.1"
operator_sdk_version="v0.13.0"

local_tmp_dir="$(pwd)/.tmp"
mkdir -p "${local_tmp_dir}"
echo "Using temporary dir: ${local_tmp_dir}"
echo

export PATH="${local_tmp_dir}:${PATH}"

echo "Downloading the kind (Kubernetes-IN-Docker)..."
kind_bin="$(download_kind ${kind_version} ${local_tmp_dir})"
echo "kind path: ${kind_bin} "
echo "kind version: $(${kind_bin} version)"
echo

cluster_name="rpaasv2-integration"
echo "Creating a Kubernetes cluster \"${cluster_name}\" with kubernetes ${KUBERNETES_VERSION}..."
create_k8s_cluster "${kind_bin}" "${cluster_name}"
echo

echo "Downloading the kubectl..."
kubectl_bin="$(download_kubectl ${KUBERNETES_VERSION}  ${local_tmp_dir})"
echo "kubectl path: ${kubectl_bin}"
echo "kubectl version: $(${kubectl_bin} version)"
echo

${kubectl_bin} config use-context "kind-${cluster_name}"

echo "Downloading the operator-sdk..."
operator_sdk_bin=$(download_operator_sdk ${operator_sdk_version} ${local_tmp_dir})
echo "operator-sdk path: ${operator_sdk_bin}"
echo "operator-sdk version: $(${operator_sdk_bin} version)"
echo

export GO111MODULE=on

echo $(which operator-sdk)

# show some info about Kubernetes cluster
${kubectl_bin} cluster-info
${kubectl_bin} get all

rpaas_system_namespace="rpaas-system"

echo "Using namespace \"${rpaas_system_namespace}\" to run \"nginx-operator\" and \"rpaas-operator\"..."
${kubectl_bin} delete namespace "${rpaas_system_namespace}" || true
${kubectl_bin} create namespace "${rpaas_system_namespace}"

run_nginx_operator "${kubectl_bin}" "${rpaas_system_namespace}" "${kind_bin}" "${cluster_name}"
run_rpaas_operator "${kubectl_bin}" "${rpaas_system_namespace}" "${kind_bin}" "${cluster_name}"

sleep 30s

${kubectl_bin} get deployment --all-namespaces
${kubectl_bin} get pods --all-namespaces

local_rpaas_api_port=39999

${kubectl_bin} -n "${rpaas_system_namespace}" port-forward svc/rpaas-api ${local_rpaas_api_port}:9999 --address=127.0.0.1 &
kubectl_port_forward_pid=${!}

sleep 10s

make build/plugin/rpaasv2

RPAAS_PLUGIN_BIN=$(pwd)/build/_output/bin/rpaasv2            \
RPAAS_API_ADDRESS="http://127.0.0.1:${local_rpaas_api_port}" \
RPAAS_OPERATOR_INTEGRATION=1                                 \
go test -test.v ./...

kill ${kubectl_port_forward_pid}

# delete_k8s_cluster "${kind_bin}" "${cluster_name}"
