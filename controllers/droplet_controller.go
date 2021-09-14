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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/buildpacks/lifecycle/launch"
	"github.com/buildpacks/lifecycle/platform"
	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pivotal/kpack/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	cfappsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
)

// DropletReconciler reconciles a Droplet object
type DropletReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	KeychainFactory registry.KeychainFactory
}

const (
	DropletReadyConditionType = "DropletReady"
)

//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=droplets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=droplets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=droplets/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Droplet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *DropletReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var droplet appsv1alpha1.Build
	logger.Info(fmt.Sprintf("Attempting to reconcile DROPLET %s", req.NamespacedName))
	// if it doesn't exist noop return
	if err := r.Get(ctx, req.NamespacedName, &droplet); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Droplet no longer exists")
		}
		logger.Info(fmt.Sprintf("Error fetching droplet: %s", err))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Decide if you keep reconciling based on statuses
	buildReadyStatusValue := getConditionOrSetAsUnknown(&droplet.Status.Conditions, cfappsv1alpha1.ReadyConditionType)
	dropletReadyStatusValue := getConditionOrSetAsUnknown(&droplet.Status.Conditions, DropletReadyConditionType)

	if buildReadyStatusValue != metav1.ConditionTrue {
		logger.Info("Build was not ready")
		return ctrl.Result{}, nil
	}

	if dropletReadyStatusValue == metav1.ConditionTrue {
		logger.Info("Droplet was ready, no need to re-reconcile")
		return ctrl.Result{}, nil
	}

	// Extract Process and Command info from build
	// Should we do this on every reconcile?
	processCommandMap, exposedPorts, err := r.extractImageConfig(ctx, logger, droplet.Status.BuildDropletStatus.Registry, droplet.Namespace)
	if err != nil {
		logger.Info(fmt.Sprintf("Error occurred extracting process types and commands: %s", err))
		return ctrl.Result{}, err
	}

	droplet.Status.BuildDropletStatus.ProcessTypes = []appsv1alpha1.ProcessType{
		processCommandMap,
	}
	droplet.Status.BuildDropletStatus.Ports = exposedPorts
	updateLocalConditionStatus(&droplet.Status.Conditions, DropletReadyConditionType, metav1.ConditionTrue, "Buildpack", "")

	// Update Build Status Conditions based on changes made to local copy
	if err := r.Status().Update(ctx, &droplet); err != nil {
		logger.Error(err, "unable to update Droplet status")
		logger.Info(fmt.Sprintf("Build status: %+v", droplet.Status))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DropletReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Build{}).
		Complete(r)
}

// fetch the Image Configuration Spec from the OCI image
// See: https://github.com/opencontainers/image-spec/blob/main/config.md
func (r *DropletReconciler) fetchImageConfig(ctx context.Context, imageRef string, imagePullSecrets []corev1.LocalObjectReference, ns string) (*v1.Config, error) {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, err
	}

	keychain, err := r.KeychainFactory.KeychainForSecretRef(ctx, registry.SecretRef{
		Namespace:        ns,
		ImagePullSecrets: imagePullSecrets,
	})
	if err != nil {
		return nil, err
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return nil, err
	}

	cfgFile, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}

	return &cfgFile.Config, nil
}

// parse the application configuration from the OCI Image Configuration
func (r *DropletReconciler) extractImageConfig(ctx context.Context, logger logr.Logger, registry appsv1alpha1.Registry, ns string) (map[string]string, []int32, error) {
	var imageConfig *v1.Config
	var err error
	exposedPorts := []int32{}

	imageConfig, err = r.fetchImageConfig(ctx, registry.Image, registry.ImagePullSecrets, ns)
	if err != nil {
		logger.Info(fmt.Sprintf("Error fetching image config: %s\n", err))
		return nil, exposedPorts, err
	}

	exposedPorts, err = extractExposedPorts(imageConfig)
	if err != nil {
		logger.Info(fmt.Sprintf("Cannot parse exposed ports from image config.. \n"))
		return nil, exposedPorts, err
	}

	// Unmarshall Build Metadata information from Image Config
	var buildMetadata platform.BuildMetadata
	err = json.Unmarshal([]byte(imageConfig.Labels[platform.BuildMetadataLabel]), &buildMetadata)
	if err != nil {
		return nil, exposedPorts, err
	}

	// Loop over all the Processes and extract the complete command string
	processCommandString := make(map[string]string)
	for _, process := range buildMetadata.Processes {
		processCommandString[process.Type] = extractFullCommand(process)
	}

	return processCommandString, exposedPorts, nil
}

// Reconstruct command with arguments into a single command string
func extractFullCommand(process launch.Process) string {
	commandWithArgs := append([]string{process.Command}, process.Args...)
	return strings.Join(commandWithArgs, " ")
}

func extractExposedPorts(imageConfig *v1.Config) ([]int32, error) {
	// Drop the protocol since we only use TCP (the default) and only store the port number
	ports := []int32{}
	for port, _ := range imageConfig.ExposedPorts {
		portInt, err := strconv.Atoi(port)
		if err != nil {
			return []int32{}, err
		}
		ports = append(ports, int32(portInt))
	}

	return ports, nil
}
