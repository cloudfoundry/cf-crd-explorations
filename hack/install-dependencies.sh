#!/usr/bin/env bash


# Make install the CF CRDs
make install

echo "******************************"
echo "Installed CF CRDS"
echo "******************************"


# CF App Validation Webhook
# generate the certs
hack/generate_certs.sh
# cert path: certs/ca.crt

# create secret
kubectl create secret generic app-validation-webhook -n default --from-file=key.pem=certs/app-validation-webhook-key.pem --from-file=cert.pem=certs/app-validation-webhook-crt.pem

# kubectl apply the webhook files but with ytt subbing out values
kubectl apply -f <(ytt -f config/webhook/app-validation.yaml --data-value "webhook_ca_cert=$(base64 -w 0 certs/ca.crt )")
kubectl apply -f config/webhook/rbac.yaml

echo "******************************"
echo "Installed and configured CF App Validating Webhook"
echo "******************************"

