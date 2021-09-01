/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"cloudfoundry.org/cf-crd-explorations/cfshim/handlers"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cfappsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=apps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=apps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=apps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the App object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch App
	app := new(cfappsv1alpha1.App)
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		logger.Info(fmt.Sprintf("Error fetching app: %s", err))
		return ctrl.Result{}, err
	}

	// If there isn't a current droplet set, don't return an error as this will cause a retry loop
	// once the app spec changes with this information, it'll reconcile then
	if app.Spec.CurrentDropletRef.Name == "" {
		return ctrl.Result{}, nil
	}

	// Fetch the Droplet to get the imageRef
	droplet := new(cfappsv1alpha1.Droplet)
	if err := r.Get(ctx, types.NamespacedName{Name: app.Spec.CurrentDropletRef.Name, Namespace: req.Namespace}, droplet); err != nil {
		logger.Info(fmt.Sprintf("Error fetching droplet: %s", err))
		return ctrl.Result{}, err
	}

	// make sure envSecret is real
	appEnvSecret := new(corev1.Secret)
	if app.Spec.EnvSecretName != "" {
		if err := r.Get(ctx, types.NamespacedName{Name: app.Spec.EnvSecretName, Namespace: req.Namespace}, appEnvSecret); err != nil {
			logger.Info(fmt.Sprintf("Error fetching appEnvSecret: %s", err))
			return ctrl.Result{}, err
		}
	}

	// fetch the LRP if it exists
	existingLRP := new(eiriniv1.LRP)
	lrpExists := true
	if err := r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: req.Namespace}, existingLRP); err != nil {
		// TODO: is there value in knowing if this was a client error or just a not-found error?
		logger.Info(fmt.Sprintf("Could not fetch LRP: %s", err))
		lrpExists = false
	}

	// This will create LRPs or remove them if not desired
	// TODO: Add desiredState stopped case
	if app.Spec.DesiredState == cfappsv1alpha1.StoppedState {
		// delete the LRP if it exists and desired state is "STOPPED"
		if lrpExists {
			err := r.Client.Delete(ctx, existingLRP)
			if err != nil {
				logger.Info(fmt.Sprintf("Error occurred deleting LRP: %s, %s", app.Name, err))
				return ctrl.Result{}, err
			}
			logger.Info(fmt.Sprintf("Successfully Deleted LRP: %s", app.Name))
		} else {
			logger.Info("Nothing to do: app desired state is \"STOPPED\"")
		}
	} else {
		// Find the default processType & its command
		var defaultProcess *cfappsv1alpha1.DropletProcessType
		for i, process := range droplet.Spec.ProcessTypes {
			if process.Default {
				defaultProcess = &droplet.Spec.ProcessTypes[i]
				break
			}
		}
		var exposedPorts []int32
		if len(droplet.Spec.Ports) == 0 {
			exposedPorts = []int32{8080}
		} else {
			exposedPorts = droplet.Spec.Ports
		}

		// update the CF App in-memory
		// if none are default, this will nil pointer
		app.Spec.ProcessType = defaultProcess.Type
		app.Spec.Command = defaultProcess.Command
		app.Spec.Ports = exposedPorts

		// Update the app CR with the droplet values
		actualAppCR := &cfappsv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name:      app.Name,
				Namespace: app.Namespace,
			},
		}
		result, err := controllerutil.CreateOrUpdate(ctx, r.Client, actualAppCR, appMutateFunction(actualAppCR, app))
		if err != nil {
			logger.Info(fmt.Sprintf("Error occurred updating App: %s, %s", result, err))
			return ctrl.Result{}, err
		}

		// Create an LRP and create/update to cluster
		desiredEiriniLRP := createAppLRP(app, droplet, appEnvSecret)
		actualEiriniLRP := &eiriniv1.LRP{
			ObjectMeta: metav1.ObjectMeta{
				Name:      app.Name,
				Namespace: app.Namespace,
			},
		}

		result, err = controllerutil.CreateOrUpdate(ctx, r.Client, actualEiriniLRP, eiriniLRPMutateFunction(actualEiriniLRP, desiredEiriniLRP))
		if err != nil {
			logger.Info(fmt.Sprintf("Error occurred updating LRP: %s, %s", result, err))
			return ctrl.Result{}, err
		}

		logger.Info(fmt.Sprintf("Successfully Created/Updated LRP: %s", result))
	}

	logger.Info("Done reconciling")
	return ctrl.Result{}, nil
}

func processMutateFunction(actualProcess, desiredProcess *cfappsv1alpha1.Process) controllerutil.MutateFn {
	return func() error {
		actualProcess.ObjectMeta.Labels = desiredProcess.ObjectMeta.Labels
		actualProcess.ObjectMeta.Annotations = desiredProcess.ObjectMeta.Annotations
		actualProcess.ObjectMeta.OwnerReferences = desiredProcess.ObjectMeta.OwnerReferences
		actualProcess.Spec = desiredProcess.Spec
		return nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cfappsv1alpha1.App{}).
		Watches(&source.Kind{Type: &cfappsv1alpha1.Droplet{}}, handler.EnqueueRequestsFromMapFunc(func(droplet client.Object) []reconcile.Request {
			var requests []reconcile.Request
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      droplet.GetLabels()[handlers.LabelAppGUID],
					Namespace: droplet.GetNamespace(),
				},
			})

			return requests
		})).
		Complete(r)
}

func createAppLRP(app *cfappsv1alpha1.App, droplet *cfappsv1alpha1.Droplet, appEnvSecret *corev1.Secret) *eiriniv1.LRP {
	// build the Deployment that we want
	desiredEiriniLRP := eiriniv1.LRP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels: map[string]string{
				handlers.LabelAppGUID:               app.Name,
				"apps.cloudfoundry.org/processGuid": app.Name,
				"apps.cloudfoundry.org/processType": app.Spec.ProcessType,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: cfappsv1alpha1.SchemeBuilder.GroupVersion.String(),
					Kind:       app.Kind,
					Name:       app.Name,
					UID:        app.UID,
				},
			},
		},
		Spec: eiriniv1.LRPSpec{
			GUID:        app.Name,
			Version:     app.ResourceVersion, // TODO: Do we care about this?
			ProcessType: app.Spec.ProcessType,
			AppName:     app.Spec.Name,
			AppGUID:     app.Name,
			OrgName:     "TBD",
			OrgGUID:     "TBD",
			SpaceName:   "TBD",
			SpaceGUID:   "TBD",
			Image:       droplet.Spec.Registry.Image,
			Command:     commandForApp(app),
			Sidecars:    nil,
			// TODO: Used for Docker images?
			//PrivateRegistry: &eiriniv1.PrivateRegistry{
			//	Username: "",
			//	Password: "",
			//},
			// TODO: Can Eirini LRP be updated to take a secret name?
			Env: secretDataToEnvMap(appEnvSecret.Data),
			Health: eiriniv1.Healthcheck{
				// TODO: Revisit int types :)
				Type:      string(app.Spec.HealthCheck.Type),
				Port:      app.Spec.Ports[0],
				Endpoint:  app.Spec.HealthCheck.Data.HTTPEndpoint,
				TimeoutMs: uint(app.Spec.HealthCheck.Data.TimeoutSeconds * 1000),
			},
			Ports:     app.Spec.Ports,
			Instances: app.Spec.Instances,
			MemoryMB:  app.Spec.MemoryMB,
			DiskMB:    app.Spec.DiskQuotaMB,
			CPUWeight: 0, // TODO: Logic in Cloud Controller is very Diego-centric. Chose not to deal with cpu requests for now
		},
	}
	return &desiredEiriniLRP
}

func commandForApp(app *cfappsv1alpha1.App) []string {
	if app.Spec.Command == "" {
		return []string{}
	} else if app.Spec.Type == cfappsv1alpha1.BuildpackLifecycle {
		return []string{"/cnb/lifecycle/launcher", app.Spec.Command}
	} else {
		return []string{"/bin/sh", "-c", app.Spec.Command}
	}
}

func appMutateFunction(actualApp, desiredApp *cfappsv1alpha1.App) controllerutil.MutateFn {
	return func() error {
		actualApp.Spec = desiredApp.Spec
		return nil
	}
}
