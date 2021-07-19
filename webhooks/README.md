# Validation Webhooks

Here you will find instructions for setting up and deploying the validation webhook for app.

## Related Docs
* [Custom Admission Controller](https://docs.giantswarm.io/advanced/custom-admission-controller/)
* [Admission Controller Response](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#response)

## Trying it out

### Development Changes
If you are making any changes to the webhook, follow instruction below to build a new image and publish to the registry

```
pack build app-validation-webhook -B paketobuildpacks/builder:full -b gcr.io/paketo-buildpacks/go --env "BP_GO_TARGETS=./webhooks"
docker tag app-validation-webhook relintdockerhubpushbot/app-validation-webhook:dev
docker push relintdockerhubpushbot/app-validation-webhook:dev
```

### Deploying Webhook

To deploy the webhook, run the `hack/install-dependencies.sh` script - it will also configure the webhook with a self-signed certificate within the namespace `default`.


Example:
```
hack/install-dependencies.sh -g "$PATH_TO_GCR_JSON"
```

### Testing the Webhook

Use the apps_v1alpha1_app.yaml from the samples directory to create an app.

```
k apply -f ./config/samples/apps_v1alpha1_app.yaml
```

Deploy another app with the same `spec.name` but different metadata by editing `metadata.name` and `metadata.labels.apps.cloudfoundry.org/appGuid` to a different value like `my-app-guid-2` and try to apply it again to see the error message.