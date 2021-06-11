# cf-crd-explorations

⚠️ **This Repository is for Experimentation Only** ⚠️

This repo contains WIP spike work done as part of our Cloud Foundry Custom Resources exploration.
It is not intended for external consumption nor are these the final definitions.
This is just a sandbox for exploring how the V3 Cloud Foundry APIs might be backed by Kubernetes Custom Resources instead of CCDB.

## Related Docs
* [Custom Resource Definition Explore](https://docs.google.com/document/d/1_3V24s81jRWQZ08M2rgTzYp1MpTtYKeDuF8vZbM72J0/edit)
* [V3 Cloud Foundry API Docs](https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html)
* [Initial High-level Design Board](https://miro.com/app/board/o9J_lFiI8CU=/)

### Trying it out

#### Installation
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

Apply sample instances of the resources
```
kubectl apply -f config/samples/. --recursive
```

#### Running the API and controllers
We currently don't support installing the API/controllers to the cluster, but you can run them locally against a targeting (via kubeconfig) K8s cluster

```
make run
```

#### Interacting with the API

```
curl localhost:81/apps | jq .
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
