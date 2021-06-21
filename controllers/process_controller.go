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
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cfappsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ProcessReconciler reconciles a Process object
type ProcessReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=processes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=processes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=processes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Process object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ProcessReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// fetch Process
	process := new(cfappsv1alpha1.Process)
	logger.Info(fmt.Sprintf("Attempting to reconcile %s", req.NamespacedName))
	if err := r.Get(ctx, req.NamespacedName, process); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Process no longer exists")
		}
		logger.Info(fmt.Sprintf("Error fetching process: %s", err))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	app := new(cfappsv1alpha1.App)
	if err := r.Get(ctx, types.NamespacedName{Name: process.Spec.AppRef.Name, Namespace: req.Namespace}, app); err != nil {
		logger.Info(fmt.Sprintf("Error fetching app: %s", err))
		return ctrl.Result{}, err
	}

	// fetch the Droplet to get the imageRef
	droplet := new(cfappsv1alpha1.Droplet)
	if err := r.Get(ctx, types.NamespacedName{Name: app.Spec.CurrentDropletRef.Name, Namespace: req.Namespace}, droplet); err != nil {
		logger.Info(fmt.Sprintf("Error fetching droplet: %s", err))
		return ctrl.Result{}, err
	}
	// may need to verify that the droplet imageRef is in the status

	appEnvSecret := new(corev1.Secret)
	if err := r.Get(ctx, types.NamespacedName{Name: app.Spec.EnvSecretName, Namespace: req.Namespace}, appEnvSecret); err != nil {
		logger.Info(fmt.Sprintf("Error fetching appEnvSecret: %s", err))
		return ctrl.Result{}, err
	}

	// build the Deployment that we want
	desiredEiriniLRP := eiriniv1.LRP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      process.Name,
			Namespace: process.Namespace,
			Labels: map[string]string{
				"apps.cloudfoundry.org/appGuid":     process.Spec.AppRef.Name,
				"apps.cloudfoundry.org/processGuid": process.Name,
				"apps.cloudfoundry.org/processType": process.Spec.ProcessType,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: cfappsv1alpha1.SchemeBuilder.GroupVersion.String(),
					Kind:       process.Kind,
					Name:       process.Name,
					UID:        process.UID,
				},
			},
		},
		Spec: eiriniv1.LRPSpec{
			GUID:        process.Name,
			Version:     process.ResourceVersion, // TODO: Do we care about this?
			ProcessType: process.Spec.ProcessType,
			AppName:     app.Spec.Name,
			AppGUID:     app.Name,
			OrgName:     "TBD",
			OrgGUID:     "TBD",
			SpaceName:   "TBD",
			SpaceGUID:   "TBD",
			Image:       droplet.Status.Image.Reference,
			Command:     commandForProcess(process, app),
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
				Type:      string(process.Spec.HealthCheck.Type),
				Port:      process.Spec.Ports[0],
				Endpoint:  process.Spec.HealthCheck.Data.HTTPEndpoint,
				TimeoutMs: uint(process.Spec.HealthCheck.Data.TimeoutSeconds * 1000),
			},
			Ports:     process.Spec.Ports,
			Instances: process.Spec.Instances,
			MemoryMB:  process.Spec.MemoryMB,
			DiskMB:    process.Spec.DiskQuotaMB,
			CPUWeight: 0, // TODO: Logic in Cloud Controller is very Diego-centric. Chose not to deal with cpu requests for now
		},
	}

	actualEiriniLRP := &eiriniv1.LRP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      process.Name,
			Namespace: process.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, actualEiriniLRP, eiriniLRPMutateFunction(actualEiriniLRP, &desiredEiriniLRP))
	if err != nil {
		logger.Info(fmt.Sprintf("Error occurred updating LRP: %s, %s", result, err))
		return ctrl.Result{}, err
	}

	logger.Info(fmt.Sprintf("Successfully Created/Updated LRP: %s", result))
	return ctrl.Result{}, nil
}

func eiriniLRPMutateFunction(actualLRP, desiredLRP *eiriniv1.LRP) controllerutil.MutateFn {
	return func() error {
		actualLRP.ObjectMeta.Labels = desiredLRP.ObjectMeta.Labels
		actualLRP.ObjectMeta.Annotations = desiredLRP.ObjectMeta.Annotations
		actualLRP.ObjectMeta.OwnerReferences = desiredLRP.ObjectMeta.OwnerReferences
		actualLRP.Spec = desiredLRP.Spec
		return nil
	}
}

func commandForProcess(process *cfappsv1alpha1.Process, app *cfappsv1alpha1.App) []string {
	if process.Spec.Command == "" {
		return []string{}
	} else if app.Spec.Type == "kpack" {
		return []string{"/cnb/lifecycle/launcher", process.Spec.Command}
	} else {
		return []string{"/bin/sh", "-c", process.Spec.Command}
	}
}

func secretDataToEnvMap(secretData map[string][]byte) map[string]string {
	convertedMap := make(map[string]string)
	for k, v := range secretData {
		convertedMap[k] = string(v)
	}
	return convertedMap
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProcessReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&cfappsv1alpha1.Process{}).
		Watches(&source.Kind{Type: &cfappsv1alpha1.App{}}, handler.EnqueueRequestsFromMapFunc(func(app client.Object) []reconcile.Request {
			processList := &cfappsv1alpha1.ProcessList{}
			_ = mgr.GetClient().List(context.Background(), processList, client.InNamespace(app.GetNamespace()), client.MatchingLabels{"apps.cloudfoundry.org/appGuid": app.GetName()})
			var requests []reconcile.Request

			for _, process := range processList.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      process.Name,
						Namespace: process.Namespace,
					},
				})
			}
			return requests
		})).
		Watches(&source.Kind{Type: &cfappsv1alpha1.Droplet{}}, handler.EnqueueRequestsFromMapFunc(func(droplet client.Object) []reconcile.Request {
			processList := &cfappsv1alpha1.ProcessList{}
			_ = mgr.GetClient().List(context.Background(), processList, client.InNamespace(droplet.GetNamespace()), client.MatchingLabels{"apps.cloudfoundry.org/appGuid": droplet.GetLabels()["apps.cloudfoundry.org/appGuid"]})
			var requests []reconcile.Request

			for _, process := range processList.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      process.Name,
						Namespace: process.Namespace,
					},
				})
			}
			return requests
		})).
		Complete(r)

	if err != nil {
		return err
	}

	return nil
}
