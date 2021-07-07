# Validation Webhooks

Here you will find instructions for setting up and deploying the validation webhook for app.

## Related Docs
* [Custom Admission Controller](https://docs.giantswarm.io/advanced/custom-admission-controller/)
* [Admission Controller Response](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#response)

## Trying it out

### Installation
#### Generate Certificates
```
./hack/generate_certs.sh
echo $CA_BUNDLE | pbcopy
```

Substitute the value for "caBundle" key in `./config/webhook/app-validation.yaml`
with what is stored in your paste buffer


We need to create a Secret to place the certificates

```
kubectl create secret generic app-validation-webhook -n default \
  --from-file=key.pem=certs/app-validation-webhook-key.pem \
  --from-file=cert.pem=certs/app-validation-webhook-crt.pem
```

### Development Changes
If you are making any changes to the webhook, follow instruction below to build a new image and publish to the registry

```
pack build app-validation-webhook -B paketobuildpacks/builder:full -b gcr.io/paketo-buildpacks/go --env "BP_GO_TARGETS=./webhooks"
docker tag app-validation-webhook relintdockerhubpushbot/app-validation-webhook:dev
docker push relintdockerhubpushbot/app-validation-webhook:dev
```

### Deploying Webhook

Note: Be sure to update the `webhooks.name` in `config/webhook/app-validation.yaml` to point to the configured apps domain for your environment. 

Before:
```
webhooks:
  - name: app-validation-webhook.INSERT_APPS_DOMAIN_HERE
```
After(example):
```
webhooks:
  - name: app-validation-webhook.beatable-spacesuit.k8s-dev.relint.rocks
```

To deploy the Webhook:

```
k apply -f ./config/webhook
```


### Testing the Webhook

Use the apps_v1alpha1_app.yaml from the samples directory to create an app.

```
k apply -f ./config/samples/apps_v1alpha1_app.yaml
```

Deploy another app with the same `spec.name` but different metadata by editing `metadata.name` and `metadata.labels.apps.cloudfoundry.org/appGuid` to a different value like `my-app-guid-2` and try to apply it again to see the error message.