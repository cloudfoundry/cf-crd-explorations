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
	"strings"

	"k8s.io/apimachinery/pkg/types"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	cfappsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"cloudfoundry.org/cf-crd-explorations/settings"

	buildv1alpha1 "github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	buildcorev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	//corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

// BuildReconciler reconciles a Build object
type BuildReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=builds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=builds/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=builds/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Build object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *BuildReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// fetch the build from the name in the Request
	var currentBuild cfappsv1alpha1.Build
	logger.Info(fmt.Sprintf("Attempting to reconcile %s", req.NamespacedName))
	// if it doesn't exist noop return
	if err := r.Get(ctx, req.NamespacedName, &currentBuild); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Build no longer exists")
		}
		logger.Info(fmt.Sprintf("Error fetching build: %s", err))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// fetch the Build's App in order to grab things like environment variables for staging
	// https://github.com/pivotal/kpack/blob/main/docs/image.md#build-configuration
	var app cfappsv1alpha1.App
	if err := r.Get(ctx, types.NamespacedName{Name: currentBuild.Spec.AppRef.Name, Namespace: req.Namespace}, &app); err != nil {
		logger.Info(fmt.Sprintf("Error fetching app: %s", err))
		// returning an error will automatically cause a retry to reconcile again later when the app is real
		return ctrl.Result{}, err
	}

	var buildPackage cfappsv1alpha1.Package
	// fetch the Build's Package
	if err := r.Get(ctx, types.NamespacedName{Name: currentBuild.Spec.PackageRef.Name, Namespace: req.Namespace}, &buildPackage); err != nil {
		logger.Info(fmt.Sprintf("Error fetching package: %s", err))
		return ctrl.Result{}, err
	}

	// Figure out if the status is succeeded True/False/Unknown
	buildSucceededStatusValue := getConditionOrSetAsUnknown(&currentBuild.Status.Conditions, cfappsv1alpha1.SucceededConditionType)
	buildStagingStatusValue := getConditionOrSetAsUnknown(&currentBuild.Status.Conditions, cfappsv1alpha1.StagingConditionType)

	// Staging not-started flow:
	//	Error and retry if package is empty
	// 		Docker: set "Staging": "False"
	//		Buildpack: create kpack Image, set "Staging": "True"
	if buildSucceededStatusValue == metav1.ConditionUnknown &&
		buildStagingStatusValue == metav1.ConditionUnknown {

		// Package empty - return ctrl with err to force retry logic
		// Indefinite retry - no exponential backoff implemented yet?
		if cfappsv1alpha1.PackageType(buildPackage.Spec.Source.Registry.Image) == "" {
			updateLocalConditionStatus(&currentBuild.Status.Conditions, cfappsv1alpha1.SucceededConditionType, metav1.ConditionUnknown, strings.Title(string(currentBuild.Spec.Type)), "packageRef package was empty")
			updateLocalConditionStatus(&currentBuild.Status.Conditions, cfappsv1alpha1.ReadyConditionType, metav1.ConditionFalse, strings.Title(string(currentBuild.Spec.Type)), "packageRef package was empty")

			var err error
			if err = r.Status().Update(ctx, &currentBuild); err != nil {
				logger.Error(err, "unable to update Build status")
				logger.Info(fmt.Sprintf("Build status: %+v", currentBuild.Status))
			} else {
				err = errors.New("packageRef package was empty")
			}
			return ctrl.Result{}, err
		}

		// For Docker type build staging, just move on to the droplet-creation stage by setting Condition "Staging": "False"
		if currentBuild.Spec.Type == cfappsv1alpha1.DockerLifecycle {
			updateLocalConditionStatus(&currentBuild.Status.Conditions, cfappsv1alpha1.ReadyConditionType, metav1.ConditionFalse, "Docker", "")
			updateLocalConditionStatus(&currentBuild.Status.Conditions, cfappsv1alpha1.StagingConditionType, metav1.ConditionFalse, "Docker", "")

			// For Buildpack type build staging, we need to create a kpack image
		} else if currentBuild.Spec.Type == cfappsv1alpha1.KPackLifecycle {
			kpackImageName := "cf-build-" + currentBuild.Name
			kpackImageNamespace := currentBuild.Namespace
			// make a desired kpack CR
			desiredKpackImage := buildv1alpha1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kpackImageName,
					Namespace: kpackImageNamespace,
					Labels: map[string]string{
						"apps.cloudfoundry.org/buildGuid": currentBuild.Name,
						"apps.cloudfoundry.org/appGuid":   app.GetName(),
					},
				},
				Spec: buildv1alpha1.ImageSpec{
					Tag: settings.GlobalSettings.RegistryTagBase + "/" + app.GetName(),
					Builder: v1.ObjectReference{
						Kind:       "Builder",
						Namespace:  kpackImageNamespace,
						Name:       "my-sample-builder", // TODO: cf-for-k8s makes a builder per-app
						APIVersion: "kpack.io/v1alpha1",
					},
					ServiceAccount: "kpack-service-account", // TODO: this is hardcoded too!
					Source: buildv1alpha1.SourceConfig{
						Registry: &buildv1alpha1.Registry{
							Image:            buildPackage.Spec.Source.Registry.Image,
							ImagePullSecrets: buildPackage.Spec.Source.Registry.ImagePullSecrets,
						},
						SubPath: "",
					},
				},
			}
			// actualImage is used by the function below to look up if we created an kpack image for this cf build already
			actualImage := &buildv1alpha1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kpackImageName,
					Namespace: kpackImageNamespace,
				},
			}
			// Actually create or update the kpack Image with K8s client
			result, err := controllerutil.CreateOrUpdate(ctx, r.Client, actualImage, cfBuildMutateFunction(actualImage, &desiredKpackImage))
			if err != nil {
				logger.Info(fmt.Sprintf("Error occurred updating kpack Image: %s, %s", result, err))
				return ctrl.Result{}, err
			}
			// after successfully creating kpack image, update the status of the build CR
			updateLocalConditionStatus(&currentBuild.Status.Conditions, cfappsv1alpha1.StagingConditionType, metav1.ConditionTrue, "Buildpack", "")
			updateLocalConditionStatus(&currentBuild.Status.Conditions, cfappsv1alpha1.ReadyConditionType, metav1.ConditionFalse, "Buildpack", "")
		}
		// END "Succeeded" == "Unknown" && "Staging" == "Unknown"

		// Staging complete("Succeeded" == "Unknown" && "Staging" == "False") flow:
		// 		Docker: create droplet from package
		//		Buildpack: create droplet from kpack build
	} else if buildSucceededStatusValue == metav1.ConditionUnknown &&
		buildStagingStatusValue == metav1.ConditionFalse {

		// These fields will be used to create the droplet later, dropletImageRegistry is made differently for Buildpack builds
		buildSucceeded := metav1.ConditionUnknown
		dropletName := "droplet-" + currentBuild.Name
		dropletNamespace := currentBuild.Namespace
		dropletImageRegistry := cfappsv1alpha1.Registry{
			Image:            buildPackage.Spec.Source.Registry.Image,
			ImagePullSecrets: buildPackage.Spec.Source.Registry.ImagePullSecrets,
		}

		if currentBuild.Spec.Type == cfappsv1alpha1.DockerLifecycle {
			// Package empty - return ctrl with err to force retry logic
			// Indefinite retry - no exponential backoff implemented yet?
			if cfappsv1alpha1.PackageType(buildPackage.Spec.Source.Registry.Image) == "" {
				updateLocalConditionStatus(&currentBuild.Status.Conditions, cfappsv1alpha1.SucceededConditionType, metav1.ConditionUnknown, "Docker", "packageRef package was empty")
				updateLocalConditionStatus(&currentBuild.Status.Conditions, cfappsv1alpha1.ReadyConditionType, metav1.ConditionFalse, "Docker", "packageRef package was empty")

				var err error
				if err = r.Status().Update(ctx, &currentBuild); err != nil {
					logger.Error(err, "unable to update Build status")
					logger.Info(fmt.Sprintf("Build status: %+v", currentBuild.Status))
				} else {
					err = errors.New("packageRef package was empty")
				}
				return ctrl.Result{}, err
			}
			buildSucceeded = metav1.ConditionTrue
		} else if currentBuild.Spec.Type == cfappsv1alpha1.KPackLifecycle {
			// look up the kpack Build CR based on the the CF build CR
			var kpackBuild buildv1alpha1.Build
			// fetch the list of kpack builds with the labels set on the kpack image we created earlier
			kpackBuildList := &buildv1alpha1.BuildList{}
			err := r.Client.List(context.Background(), kpackBuildList, client.InNamespace(currentBuild.Namespace), client.MatchingLabels{"apps.cloudfoundry.org/appGuid": app.GetName()})
			if err != nil || len(kpackBuildList.Items) == 0 {
				logger.Info(fmt.Sprintf("Error fetching kpack build for %s", app.GetName()))
				return ctrl.Result{}, err
			}
			kpackBuild = kpackBuildList.Items[0]

			// If the kpack build failed, update the CF Build CR statuses and flip the flag buildSucceeded so that a Droplet will not be created
			kpackBuildSucceededStatus := getKpackBuildCondition(&kpackBuild.Status, "Succeeded")
			if kpackBuildSucceededStatus != nil {
				buildSucceeded = stringToConditionStatus(string(kpackBuildSucceededStatus.Status))
			}
			// If the kpack build status is unknown, then retry later and return an error
			if buildSucceeded == metav1.ConditionUnknown {
				logger.Error(err, "unable to update Build status due to unknown kpack build status while Staging: False")
				return ctrl.Result{}, err
				// if the kpack build succeeded we need to create the dropletImageRegistry using the kpack build's details
			} else if buildSucceeded == metav1.ConditionTrue {
				dropletImageRegistry = cfappsv1alpha1.Registry{
					Image: kpackBuild.Status.LatestImage,
					// TODO: Ask kpack team which secrets get used to push the build- builder secret or the source image secret?
					ImagePullSecrets: kpackBuild.Spec.Source.Registry.ImagePullSecrets,
				}
			}
		}

		// buildSucceeded is usually "True" from Docker type, it is derrived from kpack build from Buildpack type
		// dropletImageRegistry is constructed from the Package for Docker type, and created from the kpack build from Buildpack type
		if buildSucceeded == metav1.ConditionTrue {
			desiredDroplet := cfappsv1alpha1.Droplet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Droplet",
					APIVersion: currentBuild.APIVersion,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      dropletName,
					Namespace: dropletNamespace,
					Labels: map[string]string{
						"apps.cloudfoundry.org/buildGuid": currentBuild.Name,
						"apps.cloudfoundry.org/appGuid":   app.GetName(),
					},
				},
				Spec: cfappsv1alpha1.DropletSpec{
					Type:   "docker",
					AppRef: currentBuild.Spec.AppRef,
					BuildRef: cfappsv1alpha1.BuildReference{
						Kind:       "Build",
						APIVersion: currentBuild.APIVersion,
						Name:       currentBuild.Name,
					},
					Registry: dropletImageRegistry,
				},
				Status: cfappsv1alpha1.DropletStatus{
					// TODO: Type is always KpackImageReference - should this have a different type for Docker images?
					ImageRef: cfappsv1alpha1.KpackImageReference{
						Kind:       buildPackage.Kind,
						APIVersion: buildPackage.APIVersion,
						Name:       buildPackage.Name,
					},
				},
			}
			actualDroplet := &v1alpha1.Droplet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dropletName,
					Namespace: dropletNamespace,
				},
			}
			// Create or update the Droplet with K8s client
			result, err := controllerutil.CreateOrUpdate(ctx, r.Client, actualDroplet, dropletMutateFunction(actualDroplet, &desiredDroplet))
			if err != nil {
				// TODO: Update build conditions when droplet push fails?
				logger.Info(fmt.Sprintf("Error occurred updating Droplet: %s, %s", result, err))
				return ctrl.Result{}, err
			}
		}
		updateLocalConditionStatus(&currentBuild.Status.Conditions, cfappsv1alpha1.SucceededConditionType, buildSucceeded, strings.Title(string(currentBuild.Spec.Type)), "")
		updateLocalConditionStatus(&currentBuild.Status.Conditions, cfappsv1alpha1.ReadyConditionType, buildSucceeded, strings.Title(string(currentBuild.Spec.Type)), "")

	}

	// Update Build Status Conditions based on changes made to local copy
	if err := r.Status().Update(ctx, &currentBuild); err != nil {
		logger.Error(err, "unable to update Build status")
		logger.Info(fmt.Sprintf("Build status: %+v", currentBuild.Status))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BuildReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cfappsv1alpha1.Build{}).
		Complete(r)
}

// watch kpack builds
// see kpack build event. if kpack build in a terminal state (completed or failed) add enqueue CF Build "request"
// reconcile the CF Build "request" we just enqueued
// look up kpack build while reconciling

// KpackBuildCFBuildReconciler flow:
// watch kpack builds
// see kpack build is in terminal state
// update status of associated build

// buildpack Build Status
// created(staging)
//		-> build reconciler makes kpack image(still staging)
//			-> kpack image makes kpack build(still staging)
//				-> kpack build completes, kpack build reconciler updates build status (staged/failed)
// 					-> build reconciler creates droplet if state == staged

// docker Build Status
// created(staging)
//		-> build reconciler just changes status (staging->staged) and exits
//			-> build reconciler creates droplet if state == staged

// The Mutate function is for only updating the fields we care about for the update CR case
func cfBuildMutateFunction(actualImage, desiredImage *buildv1alpha1.Image) controllerutil.MutateFn {
	return func() error {
		actualImage.ObjectMeta.Labels = desiredImage.ObjectMeta.Labels
		actualImage.Spec.Tag = desiredImage.Spec.Tag
		actualImage.Spec.Builder = desiredImage.Spec.Builder
		actualImage.Spec.ServiceAccount = desiredImage.Spec.ServiceAccount
		actualImage.Spec.Source = desiredImage.Spec.Source
		return nil
	}
}

func dropletMutateFunction(actualDroplet, desiredDroplet *v1alpha1.Droplet) controllerutil.MutateFn {
	return func() error {
		actualDroplet.ObjectMeta.Labels = desiredDroplet.ObjectMeta.Labels
		actualDroplet.Spec.Type = desiredDroplet.Spec.Type
		actualDroplet.Spec.AppRef = desiredDroplet.Spec.AppRef
		actualDroplet.Spec.BuildRef = desiredDroplet.Spec.BuildRef
		actualDroplet.Spec.Registry = desiredDroplet.Spec.Registry
		actualDroplet.Status.ImageRef = desiredDroplet.Status.ImageRef
		return nil
	}
}

// getConditionOrSetAsUnknown is a helper function that retrieves the value of the provided conditionType, like "Succeeded" and returns the value: "True", "False", or "Unknown"
//	if the value is not present, the pointer to the list of conditions provided to the function is used to add an entry to the list of Conditions with a value of "Unknown" and "Unknown" is returned
func getConditionOrSetAsUnknown(conditions *[]metav1.Condition, conditionType string) metav1.ConditionStatus {
	conditionStatus := meta.FindStatusCondition(*conditions, conditionType)
	conditionStatusValue := metav1.ConditionUnknown // enum for "Unknown"
	if conditionStatus != nil {
		conditionStatusValue = conditionStatus.Status
	} else {
		// set local copy of CR condition "succeeded": "unknown" because it had no value before
		meta.SetStatusCondition(conditions, metav1.Condition{
			Type:    conditionType,
			Status:  metav1.ConditionUnknown,
			Reason:  "NotReady", // TODO: Think about this. Consumers of status will care?
			Message: "",
		})
	}
	return conditionStatusValue
}

//func getCondition(buildv1alpha1.)
func getKpackBuildCondition(s *buildv1alpha1.BuildStatus, t string) *buildcorev1alpha1.Condition {
	for _, cond := range s.Conditions {
		if string(cond.Type) == t {
			return &cond
		}
	}
	return nil
}

func stringToConditionStatus(s string) metav1.ConditionStatus {
	condition := strings.ToLower(s)
	if condition == "true" {
		return metav1.ConditionTrue
	} else if condition == "false" {
		return metav1.ConditionFalse
	}
	return metav1.ConditionUnknown
}

// This is a helper function for updating local copy of status conditions
func updateLocalConditionStatus(conditions *[]metav1.Condition, conditionType string, conditionStatus metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:    conditionType,
		Status:  conditionStatus,
		Reason:  reason,
		Message: message,
	})
}
