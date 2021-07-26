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
	"errors"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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

	// A Process gets created for every process type specified in the Droplet
	var errStrings []string
	logger.Info("Starting process creation")
	for _, process := range droplet.Spec.ProcessTypes {
		for processType, command := range process {
			logger.Info("Creating process type: " + processType)
			// Default 1 for web process or Default 0
			instances := 0
			if processType == "web" {
				instances = 1
			}

			var exposedPorts []int32
			if len(droplet.Spec.Ports) == 0 {
				exposedPorts = []int32{8080}
			} else {
				exposedPorts = droplet.Spec.Ports
			}

			// TODO: This is sufficient for now. This is used to Create new processes as well as Update existing processes
			// For now let's make the "guid" a combo of app guid + process type
			// In CF for VMs there can be multiple processes of a given type so this will not work 100% of the time if we
			// we need to support that
			processGuid := fmt.Sprintf("%s-%s", app.Spec.Name, processType)
			desiredProcess := cfappsv1alpha1.Process{
				ObjectMeta: metav1.ObjectMeta{
					Name:      processGuid,
					Namespace: app.Namespace,
					Labels: map[string]string{
						"apps.cloudfoundry.org/appGuid":     app.Name,
						"apps.cloudfoundry.org/processGuid": processGuid,
						"apps.cloudfoundry.org/processType": processType,
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
				Spec: cfappsv1alpha1.ProcessSpec{
					AppRef: cfappsv1alpha1.ApplicationReference{
						Kind:       app.Kind,
						APIVersion: cfappsv1alpha1.SchemeBuilder.GroupVersion.String(),
						Name:       app.Name,
					},
					ProcessType: processType,
					Command:     command,
					State:       "STOPPED", // This is the default
					HealthCheck: cfappsv1alpha1.HealthCheck{
						// This is set to process since this information needs to be provided later on
						// API for updating health check: https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#update-a-process
						Type: "process",
					},
					Instances:   instances,
					MemoryMB:    500, // TODO: find CF default values
					DiskQuotaMB: 512, // TODO: find CF default values
					Ports:       exposedPorts,
				},
			}

			actualProcess := &cfappsv1alpha1.Process{
				ObjectMeta: metav1.ObjectMeta{
					Name:      processGuid,
					Namespace: app.Namespace,
				},
			}

			result, err := controllerutil.CreateOrUpdate(ctx, r.Client, actualProcess, processMutateFunction(actualProcess, &desiredProcess))
			if err != nil {
				logger.Info(fmt.Sprintf("Error occurred creating/updating Process: %s, %s", result, err))
				errStrings = append(errStrings, err.Error())
			}

			logger.Info(fmt.Sprintf("Successfully Created/Updated App: %s", result))
		}
	}

	// Gather all errors from Process creation and return as single error
	if len(errStrings) != 0 {
		errorStr := strings.Join(errStrings, ", ")
		err := errors.New(fmt.Sprintf("There was an error during Process creation: %s\n", errorStr))
		logger.Info(err.Error())
		return ctrl.Result{}, err
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
					Name:      droplet.GetLabels()["apps.cloudfoundry.org/appGuid"],
					Namespace: droplet.GetNamespace(),
				},
			})

			return requests
		})).
		Complete(r)
}
