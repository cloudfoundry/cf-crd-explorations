package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// ApplicationReference defines App resource that owns to this Process
type ApplicationReference struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Name       string `json:"name"`
}

// PackageReference defines Package resource that is associated to this Build
// a package gets a new build each time it is staged
type PackageReference struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Name       string `json:"name"`
}

// BuildReference defines cf Build resource that is associated to this Droplet
type BuildReference struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Name       string `json:"name"`
}

// DropletReference defines the built application image -> source code post build process
// DropletReference defines Droplet resource that is associated to a Build or App
// a package gets a new build each time it is staged
type DropletReference struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Name       string `json:"name"`
}

// ProcessType is a map of process names and associated start commands for the Droplet
type ProcessType map[string]string

// KpackImageReference is used by build
type KpackImageReference struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Name       string `json:"name"`
}

// KpackBuildReference is used by build
type KpackBuildReference struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Name       string `json:"name"`
}

// Checksum defines checksum for packaged images for now
type Checksum struct {
	Type  CheckSumType `json:"type"`
	Value string       `json:"value"`
}

// CheckSumType restrict allowed checksum types to enum
// +kubebuilder:validation:Enum=sha256;sha1
type CheckSumType string

const (
	SHA256ChecksumType = "sha256"
	SHA1ChecksumType   = "sha1"
)

// Registry is used by Package and Droplet to identify a Container Registry and secrets to access the image provided in "image"
type Registry struct {
	// image: Location of the source image
	Image string `json:"image"`
	// imagePullSecrets: A list of dockercfg or dockerconfigjson secret names required if the source image is private
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// Shared by App Lifecycle and Build
// Build can override lifecycle level definition
type LifecycleData struct {
	// List of buildpacks used to build the app with kpack
	Buildpacks []string `json:"buildpacks"`

	// Stack may be legacy from Diego, configured separately for kpack?
	Stack string `json:"stack"`
}

// These constants are for metav1 Conditons in the K8s CR Status Conditions
const (
	// the CR is ready to be consumed- for build it means a droplet has been created
	ReadyConditionType string = "Ready"
	// the CR job has completed successfully- for build set to true when droplet is created
	SucceededConditionType string = "Succeeded"
	// the build is ongoing, used for kpack builds
	StagingConditionType string = "Staging"
)

// LifecycleType inform the platform of how to build droplets and run apps
// allow only values of "docker" and "buildpack" - "buildpack" is only for cf-for-vms and is not supported
// +kubebuilder:validation:Enum=docker;buildpack
type LifecycleType string

const (
	DockerLifecycle    LifecycleType = "docker"
	BuildpackLifecycle LifecycleType = "buildpack"
)

// DesiredState used to ensure that illegal states are not provided as a string to the CRD
// +kubebuilder:validation:Enum=STARTED;STOPPED
type DesiredState string

const (
	StartedState DesiredState = "STARTED"

	StoppedState DesiredState = "STOPPED"
)
