#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# CREATE THE PRIVATE KEY FOR OUR CUSTOM CA
openssl genrsa -out certs/ca.key 2048

# GENERATE A CA CERT WITH THE PRIVATE KEY
openssl req -new -x509 -key certs/ca.key -out certs/ca.crt -config certs/ca_config.cnf

# CREATE THE PRIVATE KEY FOR OUR WEBHOOK
openssl genrsa -out certs/app-validation-webhook-key.pem 2048

# CREATE A CSR FROM THE CONFIGURATION FILE AND OUR PRIVATE KEY
openssl req -new -key certs/app-validation-webhook-key.pem -subj "/CN=app-validation-webhook.default.svc" -out certs/app-validation-webhook.csr -config certs/app-validation-webhook-config.cnf

# CREATE THE CERT SIGNING THE CSR WITH THE CA CREATED BEFORE
openssl x509 -req -in certs/app-validation-webhook.csr -CA certs/ca.crt -CAkey certs/ca.key -CAcreateserial -extensions req_ext -extfile certs/app-validation-webhook-config.cnf -out certs/app-validation-webhook-crt.pem
