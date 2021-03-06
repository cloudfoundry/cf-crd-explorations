# ⚠️ Archived ⚠️

This repo is no longer being used. Development is actively underway on https://github.com/cloudfoundry/cf-k8s-controllers and any future exploration will take place on that repo as well.

# cf-crd-explorations

⚠️ **This Repository is for Experimentation Only** ⚠️

This repo contains WIP spike work done as part of our Cloud Foundry Custom Resources exploration.
It is not intended for external consumption nor are these the final definitions.
This is just a sandbox for exploring how the V3 Cloud Foundry APIs might be backed by Kubernetes Custom Resources instead of CCDB.

## Related Docs
* [Custom Resource Definition Explore](https://docs.google.com/document/d/1_3V24s81jRWQZ08M2rgTzYp1MpTtYKeDuF8vZbM72J0/edit)
* [V3 Cloud Foundry API Docs](https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html)
* [Initial High-level Design Board](https://miro.com/app/board/o9J_lFiI8CU=/)

## Component Index
- [CRD Installation](#installation)
- [Webhook Installation](webhooks/README.md)

## Trying it out

### Setup using the install-hack script

Run the hack script to install prerequisites, CF-CRDs and app-validation webhook.

Below `PATH_TO_GCR_JSON` is a path to the file containing your registry credentials where buildpack can push built images.
```
hack/install-dependencies.sh -g "$PATH_TO_GCR_JSON"
```

It requires the following environment variables to complete installation:
- `REGISTRY_TAG_BASE`: Where buildpack built images should be published.
- `PACKAGE_REGISTRY_TAG_BASE`: The app converts packages into single layer OCI images. This is the where these images should be published.
- `REGISTRY_SECRET`: K8s secret for accessing the push/pull from package registry.

```
# Example:
export REGISTRY_TAG_BASE=gcr.io/cf-relint-greengrass/cf-crd-staging-spike/buildpack
export PACKAGE_REGISTRY_TAG_BASE="gcr.io/cf-relint-greengrass/cf-crd-staging-spike/packages"
export REGISTRY_SECRET="app-registry-credentials"
```

To run locally against a targeted K8s cluster, jump to `Running locally` section

To run on a cluster, jump to `Run on Cluster` section

### Cluster Pre-requisites
The below requirements can be installed with the `./hack/install-dependencies.sh` script. It takes a flag to a gcr json key with a `-g` or `--gcr-service-account-json` flag, which specifies the file location of a gcr json key.

* Eirini Controller installed ([instructions](https://github.com/cloudfoundry-incubator/eirini-controller/blob/master/README.md))
* Kpack installed ([instructions](https://github.com/pivotal/kpack/blob/main/docs/install.md))
  * Follow the [kpack tutorial](https://github.com/pivotal/kpack/blob/main/docs/tutorial.md) for more information on how to set it up
  * Sample kpack resources are available in `config/samples/kpack`


### Installation
Clone this repo:
```
cd ~/workspace
git clone git@github.com:cloudfoundry/cf-crd-explorations.git
cd cf-crd-explorations/
```

Deploy CRDs to K8s in current Kubernetes context
```
make install
```

Install prerequisites, kpack & eirini, as well as the validating webhook for cf apps:

* If on Mac, you should confirm that your openssl version is > `3.0` with `openssl version`
  * To install the latest openssl version on Mac run:
    ```  
    brew install libressl
    echo 'export PATH="/usr/local/opt/libressl/bin:$PATH"' >> ~/.bash_profile
    export PATH="/usr/local/opt/libressl/bin:$PATH"
    ```


## Running the API and controllers

### Running locally
Run controllers locally against a targeting (via kubeconfig) K8s cluster

The spike code converts Apps, Processes, and Droplets into kubernetes resources, including Eirini LRP resources which
require the Eirini LRP controller (see cluster pre-requisites above for information on how to install it).

It also produces staged Droplets from Packages and Builds using kpack.

To start the controller locally, run:

```
make run
```

Apply sample instances of the resources.
```
kubectl apply -f config/samples/cf-crds/. --recursive
kubectl apply -f config/samples/supporting-objects/app_env_secret.yaml
```

**Note:** If you want the sample app to be routable you must update the sample Route CR (config/samples/sample_app_route.yaml) to point to the configured apps domain for your environment. Since we're leveraging cf-for-k8s for its Eirini installation the easiest way to make the app routable is by using the existing cf-for-k8s RouteController and Route CR.

### Run on Cluster

The deployment spec for the controller will need to have the `REGISTRY_TAG_BASE` env var set in order for the controller to understand where to publish images. See: https://kubernetes.io/docs/tasks/inject-data-application/define-environment-variable-container/

As when deploying locally, kpack and Eirini will need to be configured and deployed.

If code changes are made, the controller manager image will also need to be built and pushed to a registry via the make commands.
```
make docker-build
make docker-push
```


To deploy to the controller manager to the cluster, run:
```
make deploy
```

In order to access the API shim, you need to configure a service such as `config/supporting-objects/service.yaml`. Once configured, you can curl the available endpoints.

For example:
```
curl LB_IP/v3/apps/
```

### Manually update the ImageRef on the sample Droplet
Since we do not have a spike implementation of staging or a Droplets Controller at this time (we expect to do this in https://github.com/cloudfoundry/cf-crd-explorations/issues/6), we have to manually set the image on the sample Droplet. To do this you must 

1. `kubectl proxy &`
2.
```
NAMESPACE=cf-workloads
DROPLET_NAME=buildpack-droplet-guid

curl -k -s -X PATCH -H "Accept: application/json, */*" \
-H "Content-Type: application/merge-patch+json" \
127.0.0.1:8001/apis/apps.cloudfoundry.org/v1alpha1/namespaces/$NAMESPACE/droplets/$DROPLET_NAME/status \
--data '{"status":{"image": {"reference": "relintdockerhubpushbot/dora", "pullSecretName": ""}, "conditions": []}}'
```

### Interacting with the API
To experiment with the CF API shim, you can access the following endpoints and actions.

|       ACTION       |        URL                                           |
|--------------------|------------------------------------------------------|
| **GET** / **POST** | `/v3/apps`                                           |
| **GET** / **PUT**  | `/v3/apps/:guid`                                     |
| **POST**           | `/v3/packages`                                       |
| **GET**            | `/v3/packages/:guid`                                 |
| **POST**           | `/v3/packages/:guid/upload`                          |
| **GET**            | `/v3/builds`                                         |
| **GET**            | `/v3/builds/:guid`                                   |
| **PATCH**          | `/v3/apps/:guid/relationships/current_droplet`       |
| **POST**           | `/v3/apps/:guid/actions/<start/stop>`                |


For example, you can get a list of applications by running `curl http://localhost:9000/v3/apps | jq .`

#### Filtering Results
The `/v3/apps` endpoint allows filtering.

```
$ curl http://localhost:9000/v3/apps?lifecycle_type=buildpack
$ curl http://localhost:9000/v3/apps?names=my-app-name,<new spec.name>
```

Note: non-existent filter fields will not restrict results. In the case of a bogus filter, all results will be returned. We should discuss what our intended behavior is in the future.

#### Creating or Updating Apps
```
curl "http://localhost:9000/v3/apps" \
  -X POST \
  -d '{"name":"my-app","relationships":{"space":{"data":{"guid":"cf-workloads"}}}}'
```


```
curl "http://localhost:9000/v3/apps/9f924342-472a-43a1-9db9-54beba5401e2" \
  -X PUT \
  -d '{"name":"my-app","lifecycle":{"type":"buildpack","data":{"buildpacks":["java_buildpack","ruby"],"stack":"cflinuxfs3"}}}'
```

#### Creating Packages and Uploading Bits Package
In order to create a docker Package, the associated App must be created first.

```
curl "http://localhost:9000/v3/packages" \
  -X POST \
  -d '{"type":"docker","relationships":{"app":{"data":{"guid":"9f924342-472a-43a1-9db9-54beba5401e2"}}},"data":{"image":"registry/your-image:latest","username":"dockerusername","password":"dockerpassword"}}'

```

To create a bits package and then upload source code via the following:

```
curl "http://localhost:9000/v3/packages" \
  -X POST \
  -d '{"type": "bits","relationships":{"app":{"data":{"guid":"9f924342-472a-43a1-9db9-54beba5401e2"}}}}'

```

```curl "http://localhost:9000/v3/packages/11c5d0ae-3bc6-441d-ac79-2ebd53b421c9/upload" \
  -X POST \
  -F bits=@"<path to zip>"
```

#### Creating Builds

```
curl "http://localhost:9000/v3/builds" \
  -X POST \
  -d '{"package": {"guid": "11c5d0ae-3bc6-441d-ac79-2ebd53b421c9"}}'

```

#### Adding the Current Droplet to the App

Update the App to set the current droplet.

```
curl "http://localhost:9000/v3/apps/9f924342-472a-43a1-9db9-54beba5401e2/relationships/current_droplet" \
  -X PATCH \
  -d '{"data":{"guid": "3874fb78-a0da-414a-ad6f-b2c18e904e57"}}'

```

#### Starting/Stopping the App

To start or stop the App

```
curl "http://localhost:9000/v3/apps/9f924342-472a-43a1-9db9-54beba5401e2/actions/start" \
  -X POST
```

```
curl "http://localhost:9000/v3/apps/9f924342-472a-43a1-9db9-54beba5401e2/actions/stop" \
  -X POST
```

---

### Developing

#### Making Changes to the CRDs

* Golang CR Definitions live in `api/v1alpha1/`
* Apply Changes to re-generate K8s CR Manifests
```
make manifests
```
