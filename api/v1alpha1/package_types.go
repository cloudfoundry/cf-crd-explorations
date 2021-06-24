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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PackageSpec defines the desired state of Package
type PackageSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Specifies the package type, either bits or docker
	// Valid values are:
	// "bits": package to upload source code
	// "docker": package references a docker image from a registry
	Type PackageType `json:"type"`

	// Specifies the App that owns this package
	AppRef ApplicationReference `json:"appRef"`

	// Contains the details for the source image(bits) or docker image(docker)
	Source PackageSource `json:"source"`
}

type PackageSource struct {
	// registry ( Source code is an OCI image in a registry that contains application source)
	Registry Registry `json:"registry"`

	// subPath: A subdirectory within the source folder where application code resides. Can be ignored if the source code resides at the root level.
	SubPath string `json:"subPath,omitempty"`
}

// PackageType used to enum the inputs to package.type
// +kubebuilder:validation:Enum=bits;docker
type PackageType string

const (
	BitsPackage   PackageType = "bits"
	DockerPackage PackageType = "docker"
)

// PackageStatus defines the observed state of Package
type PackageStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Contains the checksum for the packaged source code image
	Checksum Checksum `json:"checksum,omitempty"`

	// Contains the current status of the package
	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Package is the Schema for the packages API
type Package struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackageSpec   `json:"spec,omitempty"`
	Status PackageStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PackageList contains a list of Package
type PackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Package `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Package{}, &PackageList{})
}
