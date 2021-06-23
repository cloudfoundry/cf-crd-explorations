# cf-crd-explorations

⚠️ **This Repository is for Experimentation Only** ⚠️

This repo contains WIP spike work done as part of our Cloud Foundry Custom Resources exploration.
It is not intended for external consumption nor are these the final definitions.
This is just a sandbox for exploring how the V3 Cloud Foundry APIs might be backed by Kubernetes Custom Resources instead of CCDB.

## Related Docs
* [Custom Resource Definition Explore](https://docs.google.com/document/d/1_3V24s81jRWQZ08M2rgTzYp1MpTtYKeDuF8vZbM72J0/edit)
* [V3 Cloud Foundry API Docs](https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html)
* [Initial High-level Design Board](https://miro.com/app/board/o9J_lFiI8CU=/)

## Trying it out

### Installation
Clone this repo:
```
cd ~/workspace
git clone git@github.com:cloudfoundry/cf-crd-explorations.git
cd cf-crd-explorations/
```

Deploy CRs to K8s in current Kubernetes context
```
make install
```

### Running the API and controllers
We currently don't support installing the API/controllers to the cluster, but you can run them locally against a targeting (via kubeconfig) K8s cluster

The spike code converts Apps, Processes, and Droplets into Eirini LRP resources which requires the Eirini LRP controller to be deployed to the cluster. The simplest way to do this right now is to deploy cf-for-k8s to the cluster using the [eirini-controller-enabled branch](https://github.com/cloudfoundry/cf-for-k8s/tree/eirini-controller-enabled). Follow the standard cf-for-k8s installation steps to do so.

```
make run
```

Apply sample instances of the resources.
```
kubectl apply -f config/samples/. --recursive
```

**Note:** If you want the sample app to be routable you must update the sample Route CR (config/samples/sample_app_route.yaml) to point to the configured apps domain for your environment. Since we're leveraging cf-for-k8s for its Eirini installation the easiest way to make the app routable is by using the existing cf-for-k8s RouteController and Route CR.

#### Manually update the ImageRef on the sample Droplet
Since we do not have a spike implementation of staging or a Droplets Controller at this time (we expect to do this in https://github.com/cloudfoundry/cf-crd-explorations/issues/6), we have to manually set the image on the sample Droplet. To do this you must 

1. `kubectl proxy &`
2.
```
NAMESPACE=cf-workloads
DROPLET_NAME=kpack-droplet-guid

curl -k -s -X PATCH -H "Accept: application/json, */*" \
-H "Content-Type: application/merge-patch+json" \
127.0.0.1:8001/apis/apps.cloudfoundry.org/v1alpha1/namespaces/$NAMESPACE/droplets/$DROPLET_NAME/status \
--data '{"status":{"image": {"reference": "relintdockerhubpushbot/dora", "pullSecretName": ""}, "conditions": []}}'
```

### Interacting with the API
To experiment with the CF API shim, you can access the following endpoints and actions.

|       ACTION       |        URL       |
|--------------------|------------------|
| **GET** / **POST** | `/v3/apps`       |
| **GET** / **PUT**  | `/v3/apps/:guid` |
| **POST**           | `/v3/packages`   |

For example, you can get a list of applications with `http://localhost:81/v3/apps | jq .`

#### Filtering Results
The `/v3/apps` endpoint allows filtering.

```
$ curl http://localhost:81/v3/apps?lifecycle_type=kpack
$ curl http://localhost:81/v3/apps?names=my-app-name,<new spec.name>
```

Note: non-existent filter fields will not restrict results. In the case of a bogus filter, all results will be returned. We should discuss what our intended behavior is in the future.

#### Creating or Updating Apps
```
curl "http://localhost:81/v3/apps/9f924342-472a-43a1-9db9-54beba5401e2" \
  -X PUT \
  -d '{"name":"my-app","lifecycle":{"type":"kpack","data":{"buildpacks":["java_buildpack","ruby"],"stack":"cflinuxfs3"}}}'
```

#### Creating Packages
In order to create a docker Package, the associated App must be created first.

```
curl "http://localhost:81/v3/packages" \
  -X POST \
  -d '{"type":"docker","relationships":{"app":{"data":{"guid":"9f924342-472a-43a1-9db9-54beba5401e2"}}},"data":{"image":"registry/your-image:latest","username":"dockerusername","password":"dockerpassword"}}'

```

---

### Developing

#### Making Changes to the CRDs

* Golang CR Definitions live in `api/v1alpha1/`
* Apply Changes to re-generate K8s CR Manifests
```
make manifests
```

**NOTE:**

This will generate a file called `cf-crd-explorations/config/crd/bases/apps.cloudfoundry.org_buildren.yaml` with kubebuilder 3.1.0 you need to modify this file and rename it for the proper plural of build -> -buildren- builds to appear in K8s.
Refer to `cf-crd-explorations/config/crd/bases/apps.cloudfoundry.org_builds.yaml` for an example.
