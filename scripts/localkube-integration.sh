#!/bin/bash -e

download_kubectl() {
  local version="${1:-"v1.14.1"}"
  local destination="${2:-".tmp"}"

  local kubectl_bin="${destination}/kubectl"

  [[ -f "${kubectl_bin}" ]] && echo -n "${kubectl_bin}" && return

  local os="$(get_os)"
  local arch="$(get_arch)"

  curl -sLo ${kubectl_bin} "https://storage.googleapis.com/kubernetes-release/release/${version}/bin/${os}/${arch}/kubectl" && \
  chmod +x ${kubectl_bin}
  echo -n ${kubectl_bin}
}

download_kind() {
  local version="${1:-"v0.3.0"}"
  local destination="${2:-".tmp"}"

  local kind_bin="${destination}/kind"

  [[ -f "${kind_bin}" ]] && echo -n "${kind_bin}" && return

  local os="$(get_os)"
  local arch="$(get_arch)"

  curl -sLo "${kind_bin}" "https://github.com/kubernetes-sigs/kind/releases/download/${version}/kind-${os}-${arch}" && \
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

  ${kind_bin} create cluster \
    --name "${cluster_name}" --wait "10m" \
    --image "kindest/node:${KUBERNETES_VERSION:-"v1.14.1"}"
}

delete_k8s_cluster() {
  local kind_bin="${1}"
  local name=${2}

  ${kind_bin} delete cluster --name "${name}"
}

run_nginx_operator() {
  local kubectl_bin="${1}"
  local namespace="${2}"
  local cluster_name="${3}"
  local tag="${4:-"integration"}"


  local nginx_operator_module_dir=$(go mod download -json github.com/tsuru/nginx-operator | jq .Dir | tr -d '""')
  local nginx_operator_dir=${GOPATH}/src/github.com/tsuru/nginx-operator

  cp -R ${nginx_operator_module_dir} ${nginx_operator_dir}

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

  echo tsuru/rpaas-{api,operator} | tr ' ' '\n' |
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

[[ -n ${DEBUG} ]] && set -x

# When KUBERNETES_VERSION isn't defined, use v1.14.1
[[ -z ${KUBERNETES_VERSION} ]] && export KUBERNETES_VERSION="v1.14.1"

local_tmp_dir="$(pwd)/.tmp"
mkdir -p "${local_tmp_dir}"
echo "Using temporary dir: ${local_tmp_dir}"
echo

kind_version="v0.3.0"
echo "Downloading the kind (Kubernetes-IN-Docker)..." && \
kind_bin="$(download_kind ${kind_version} ${local_tmp_dir})" && \
echo "kind path: ${kind_bin} " && \
echo "kind version: ${kind_version}" && \
echo

cluster_name="rpaasv2-integration"
echo "Creating a Kubernetes cluster \"${cluster_name}\"..." && \
create_k8s_cluster "${kind_bin}" "${cluster_name}" && \
echo "Kubernetes version: ${KUBERNETES_VERSION}"
echo

echo "Downloading the kubectl..." && \
kubectl_bin="$(download_kubectl)" && \
kubeconfig="$(${kind_bin} get kubeconfig-path --name ${cluster_name})" && \
echo "kubectl path: ${kubectl_bin}" && \
echo "kubectl version: ${KUBERNETES_VERSION}" && \
echo "kubeconfig path: ${kubeconfig}" && \

export KUBECONFIG="${kubeconfig}"

# show some info about Kubernetes cluster
${kubectl_bin} cluster-info && \
${kubectl_bin} get all

rpaas_system_namespace="rpaas-system"

echo "Using namespace \"${rpaas_system_namespace}\" to run \"nginx-operator\" and \"rpaas-operator\"..."
${kubectl_bin} create namespace "${rpaas_system_namespace}"

run_nginx_operator "${kubectl_bin}" "${rpaas_system_namespace}" "${cluster_name}"
run_rpaas_operator "${kubectl_bin}" "${rpaas_system_namespace}" "${kind_bin}" "${cluster_name}"

sleep 30s

${kubectl_bin} get deployment --all-namespaces
${kubectl_bin} get pods --all-namespaces

local_rpaas_api_port=39999

${kubectl_bin} -n "${rpaas_system_namespace}" port-forward svc/rpaas-api ${local_rpaas_api_port}:9999 --address=127.0.0.1 &
kubectl_port_forward_pid=${!}

sleep 10s

PATH="${PATH}:$(pwd)/.tmp"                                   \
RPAAS_API_ADDRESS="http://127.0.0.1:${local_rpaas_api_port}" \
RPAAS_OPERATOR_INTEGRATION=1                                 \
go test ./...

kill ${kubectl_port_forward_pid}

delete_k8s_cluster "${kind_bin}" "${cluster_name}"
