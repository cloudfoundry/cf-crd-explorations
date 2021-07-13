#!/usr/bin/env bash

set -euo pipefail

function usage_text() {
  cat <<EOF
Usage:
  $(basename "$0")

flags:
  -g, --gcr-service-account-json
      (optional) Filepath to the GCP Service Account JSON describing a service account
      that has permissions to write to the project's container repository.

EOF
  exit 1
}

if [[ $# -lt 1 ]]; then
  usage_text >&2
fi

while [[ $# -gt 0 ]]; do
  i=$1
  case $i in
  -g=* | --gcr-service-account-json=*)
    GCP_SERVICE_ACCOUNT_JSON_FILE="${i#*=}"
    shift
    ;;
  -g | --gcr-service-account-json)
    GCP_SERVICE_ACCOUNT_JSON_FILE="${2}"
    shift
    shift
    ;;
  *)
    echo -e "Error: Unknown flag: ${i/=*/}\n" >&2
    usage_text >&2
    exit 1
    ;;
  esac
done

# For GCR with a json key, DOCKER_USERNAME is `_json_key`
DOCKER_USERNAME=${DOCKER_USERNAME:-'_json_key'}
DOCKER_PASSWORD=$(cat $GCP_SERVICE_ACCOUNT_JSON_FILE)
DOCKER_SERVER="${DOCKER_SERVER:-'gcr.io'}"

# Kpack
kubectl create secret docker-registry tutorial-registry-credentials \
    --docker-username=$DOCKER_USERNAME \
    --docker-password="$DOCKER_PASSWORD" \
    --docker-server=$DOCKER_SERVER \
    --namespace default

kubectl apply -f config/samples/kpack/release-0.3.1.yaml
kubectl apply -f config/samples/kpack/serviceaccount.yaml \
    -f config/samples/kpack/stack.yaml \
    -f config/samples/kpack/store.yaml \
    -f config/samples/kpack/builder.yaml

echo "******************************"
echo "Installed and configured Kpack"
echo "******************************"

# Eirini
kubectl create ns eirini-controller
kubectl create ns cf-workloads

curl https://raw.githubusercontent.com/cloudfoundry-incubator/eirini-controller/master/deployment/scripts/generate-secrets.sh | bash -s - "*.eirini-controller.svc"

VERSION=0.1.0
WEBHOOK_CA_BUNDLE="$(kubectl get secret -n eirini-controller eirini-instance-index-env-injector-certs -o jsonpath="{.data['tls\.ca']}")"
RESOURCE_VALIDATOR_CA_BUNDLE="$(kubectl get secret -n eirini-controller eirini-resource-validator-certs -o jsonpath="{.data['tls\.ca']}")"

helm install eirini-controller https://github.com/cloudfoundry-incubator/eirini-controller/releases/download/v$VERSION/eirini-controller-$VERSION.tgz \
  --namespace eirini-controller \
  --set "webhook_ca_bundle=$WEBHOOK_CA_BUNDLE" \
  --set "resource_validator_ca_bundle=$RESOURCE_VALIDATOR_CA_BUNDLE"

echo "******************************"
echo "Installed and configured Eirini"
echo "******************************"
