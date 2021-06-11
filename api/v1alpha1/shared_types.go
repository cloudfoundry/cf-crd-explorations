package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

// +kubebuilder:validation:Enum=True;False;Unknown
type ConditionStatus string

const (
	TrueConditionStatus    ConditionStatus = "True"
	FalseConditionStatus   ConditionStatus = "False"
	UnknownConditionStatus ConditionStatus = "Unknown"
)

// TODO: Double check that we are using the correct the condition type from kubernetes
// Loosely following this KEP:
// https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/1623-standardize-conditions
// Eventually we can update to use standard Kubernetes types
type Condition struct {
	Type               string          `json:"type"`
	Status             ConditionStatus `json:"status"`
	LastTransitionTime metav1.Time     `json:"lastTransitionTime"`
	Reason             string          `json:"reason"`
	Message            string          `json:"message"`
}

// Shared by App Lifecycle and Build
// Build can override lifecycle level definition
type LifecycleData struct {
	// List of buildpacks used to build the app with kpack
	Buildpacks []string `json:"buildpacks"`

	// Stack may be legacy from Diego, configured separately for kpack?
	Stack string `json:"stack"`
}

// LifecycleType inform the platform of how to build droplets and run apps
// allow only values of "docker" and "kpack" - "buildpack" is only for cf-for-vms and is not supported
// +kubebuilder:validation:Enum=docker;kpack
type LifecycleType string

const (
	DockerLifecycle LifecycleType = "docker"
	KPackLifecycle  LifecycleType = "kpack"
)

// DesiredState used to ensure that illegal states are not provided as a string to the CRD
// +kubebuilder:validation:Enum=STARTED;STOPPED
type DesiredState string

const (
	StartedState DesiredState = "STARTED"

	StoppedState DesiredState = "STOPPED"
)
