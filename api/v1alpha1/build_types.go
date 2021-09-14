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

// EDIT THIS FILE! THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required. Any new fields you add must have json tags for the fields to be serialized.
// BuildSpec defines the desired state of Build
type BuildSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// Specifies the lifecycle type kpack or docker of the build
	Type LifecycleType `json:"type"`
	// Specifies the Package associated with this build
	PackageRef PackageReference `json:"packageRef"`
	// Specifies the App associated with this build
	AppRef ApplicationReference `json:"appRef"`
	// Specifies the buildpacks and stack of the build, empty for docker
	LifecycleData LifecycleData `json:"lifecycleData,omitempty"`
	// Optional, Links kpack builds explicitly
	KpackBuildSelector KpackBuildSelector `json:"kpackBuildSelector,omitempty"`
	// Optional, specify labels to put on generated kpack image
	KpackImageTemplate KpackImageTemplate `json:"kpackImageTemplate,omitempty"`
}
type KpackBuildSelector struct {
	MatchLabels map[string]string `json:"matchLabels"`
}
type KpackImageTemplate struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// BuildStatus defines the observed state of Build
type BuildStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// Contains a reference to the compiled build image
	DropletReference DropletReference `json:"dropletRef,omitempty"`

	// TODO: figure out why omitempty behaves weird, seems like kubectl doesn't even represent internally with an empty slice
	// Contains the current status of the build
	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// Build is the Schema for the builds API
//+kubebuilder:resource:shortName=cfb;cfbuild
type Build struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              BuildSpec   `json:"spec,omitempty"`
	Status            BuildStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// BuildList contains a list of Build
type BuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Build `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Build{}, &BuildList{})
}
