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

	cfappsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	"github.com/go-logr/logr"
	buildv1alpha1 "github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const BuildGUIDLabel = "apps.cloudfoundry.org/buildGuid"
const BuildReasonAnnotation = "image.kpack.io/reason"
const StackUpdateBuildReason = "STACK"

// CFKpackBuildReconciler reconciles a AppManifest object
type CFKpackBuildReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *CFKpackBuildReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var kpackBuild buildv1alpha1.Build

	logger.Info(fmt.Sprintf("Attempting to reconcile %s", req.NamespacedName))
	if err := r.Get(ctx, req.NamespacedName, &kpackBuild); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Kpack Build no longer exists")
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var cfBuild cfappsv1alpha1.Build
	if err := r.Get(ctx, types.NamespacedName{Name: kpackBuild.ObjectMeta.Labels[BuildGUIDLabel], Namespace: req.Namespace}, &cfBuild); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("CF Build no longer exists")
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	condition := kpackBuild.Status.GetCondition(corev1alpha1.ConditionSucceeded)
	if condition.IsTrue() {
		return r.reconcileSuccessfulBuild(ctx, &kpackBuild, &cfBuild, logger)
	}

	failureMessage := fmt.Sprintf(
		"Kpack build unsuccessful: Build failure reason: '%s', message: '%s'.",
		condition.Reason,
		condition.Message,
	)

	failedContainerState := findAnyFailedContainerState(kpackBuild.Status.StepStates)
	if failedContainerState != nil {
		failureMessage = fmt.Sprintf(
			"Kpack build failed during container execution: Step failure reason: '%s', message: '%s'.",
			failedContainerState.Terminated.Reason,
			failedContainerState.Terminated.Message,
		)
	}

	return r.reconcileFailedBuild(ctx, &kpackBuild, &cfBuild, failureMessage, logger)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CFKpackBuildReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	logger := log.FromContext(ctx)

	return ctrl.NewControllerManagedBy(mgr).
		For(&buildv1alpha1.Build{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				logger.WithValues("requestLink", e.Object.GetSelfLink()).
					V(1).Info("Kpack Build create event received")
				return buildFilter(e.Object)
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				logger.WithValues("requestLink", e.ObjectNew.GetSelfLink()).
					V(1).Info("Kpack Build update event received")
				return buildFilter(e.ObjectNew)
			},
			DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
			GenericFunc: func(_ event.GenericEvent) bool { return false },
		}).
		Complete(r)
}

var BuildFilterError = errors.New("Received a build event with a non-build runtime.Object")

func buildFilter(e runtime.Object) bool {
	ctx := context.Background()
	logger := log.FromContext(ctx)

	newBuild, ok := e.(*buildv1alpha1.Build)
	if !ok {
		logger.WithValues("event", e).Error(BuildFilterError, "ignoring event")
		return false
	}

	if _, isGuidPresent := newBuild.ObjectMeta.Labels[BuildGUIDLabel]; !isGuidPresent {
		logger.WithValues("build", newBuild).V(1).Info("ignoring event: received update event for a non-CF Build resource")
		return false
	}
	buildReason, ok := newBuild.ObjectMeta.Annotations[BuildReasonAnnotation]
	if !ok {
		logger.WithValues("build", newBuild).V(1).Info("ignoring event: received update event that was missing the build reason")
		return false
	}

	// Ignoring builds triggered by Stack updates for now
	if buildReason == StackUpdateBuildReason {
		logger.WithValues("build", newBuild).V(1).Info("ignoring event: build triggered due to an automatic stack update")
		return false
	}

	// Wait until the 'Succeeded' condition is in a terminal 'False' or 'True' state
	if newBuild.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsUnknown() {
		logger.WithValues("build", newBuild).V(1).Info("ignoring event: build 'Succeeded' condition status is Unknown")
		return false
	}

	logger.WithValues("build", newBuild).V(1).Info("event passed ignore filters, continuing with reconciliation")
	return true
}

func (r *CFKpackBuildReconciler) reconcileSuccessfulBuild(ctx context.Context, kpackBuild *buildv1alpha1.Build, cfBuild *cfappsv1alpha1.Build, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Kpack Build completed successfully")

	meta.SetStatusCondition(&cfBuild.Status.Conditions, metav1.Condition{
		Type:    cfappsv1alpha1.StagingConditionType,
		Status:  metav1.ConditionFalse,
		Reason:  "Succeeded",
		Message: "",
	})

	if err := r.Status().Update(ctx, cfBuild); err != nil {
		logger.Error(err, "unable to update Build status")
		logger.Info(fmt.Sprintf("Build status: %+v", cfBuild.Status))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *CFKpackBuildReconciler) reconcileFailedBuild(ctx context.Context, kpackBuild *buildv1alpha1.Build, cfBuild *cfappsv1alpha1.Build, errorMessage string, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Kpack Build failed")

	meta.SetStatusCondition(&cfBuild.Status.Conditions, metav1.Condition{
		Type:    cfappsv1alpha1.StagingConditionType,
		Status:  metav1.ConditionFalse,
		Reason:  "Failed",
		Message: errorMessage,
	})

	if err := r.Status().Update(ctx, cfBuild); err != nil {
		logger.Error(err, "unable to update Build status")
		logger.Info(fmt.Sprintf("Build status: %+v", cfBuild.Status))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// returns true if any container has terminated with a non-zero exit code
func findAnyFailedContainerState(containerStates []corev1.ContainerState) *corev1.ContainerState {
	for _, container := range containerStates {
		if container.Terminated != nil && container.Terminated.ExitCode != 0 {
			return &container
		}
	}
	return nil
}
