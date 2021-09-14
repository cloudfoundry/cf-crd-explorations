## Assumptions
### Miro Board
https://miro.com/app/board/o9J_lwiBJ3c=/

* Operator is responsible for setting up Namespaces and ServiceAccounts + default Builders for each Namespace.
    * SA and Builder names should follow a convention: `${namespace}-kpack-service-account` & `${namespace}-kpack-default-builder`
* We won't be cleaning up Builders or their associated images at this point. This should be done eventually, but is not an MVP concern.
* If a lifecycle is specified for a Build at a certain point, then unspecified later, the App will be built with the next higher precedence lifecycle `build lifecycle > app lifecycle > default lifecycle`
* Build controller will create and monitor Kpack Builders.


Path 1 - Default Builder
1. App created with empty lifecycle
1. Build created with empty lifecycle
1. Default builder is used.

Path 2 - App specifies Lifecycle
1. App created with lifecycle
1. Build created with empty lifecycle
1. Build controller looks up App lifecycle
1. App builder is fetched or created.
    1. If it exists, create Kpack Image with referenced Builder
    1. If it does not exist, a Builder is created and labeled with App GUID *only*

Path 3 - Build specifies Lifecycle
1. App created with empty lifecycle
1. Build created with lifecycle
1. Build controller uses Build lifecycle
1. Build builder is created and labeled with Build GUID *only*

Path 4 - App and Build specify Lifecycles
1. App created with lifecycle
1. Build created with lifecycle
1. Build controller ignores App lifecycle in favor of Build lifecycle
1. Build builder is created and labeled with Build GUID *only*


## Suggestions
* Add new status to Build to reflect the status of the Builder (created, not ready, ready, etc).
* Build controller watches Kpack for Builder updates; when a Builder is ready, the controller can look up the associated Build from the labels on the Builder.


## Outcomes
 Additionally, please consider and include on the following questions:

> If a default, superset Builder is used, how will it be configured, via ConfigMap or otherwise?

* We are currently proposing that it is the operator's responsibility to configure the default builders. They can use whatever tooling they desire.
* It does not appear that Builders can be configured with Buildpacks via ConfigMap.

> What implications does the proposed strategy have on buildpack/stack upgrades? Please include a high-level summary of the user actions required to roll out such an upgrade.

* This should be answered by members of the Buildpacks, Kpack, or TBS teams.

> What modifications would we have to make to the staging Controllers and Webhooks to allow for users to build images in any CF Spaces they have access to, and push them to the per-Org/per-Space registry configured by the platform operator?

* The operator would need to specify the proper credentials per namespace.
* Some changes might need to be made to change how SA & Secrets are loaded since they are assumed to be Globalish today (in this spike repo).