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

// DropletSpec defines the desired state of Droplet
type DropletSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Specifies the Lifecycle type buildpack or docker of the droplet
	Type LifecycleType `json:"type"`

	// Specifies the App associated with this Droplet
	AppRef ApplicationReference `json:"appRef"`

	// Specifies the Build associated with this Droplet
	BuildRef BuildReference `json:"buildRef"`

	// Specifies the Container registry image, and secrets to access
	Registry Registry `json:"registry,omitempty"`

	// Specifies the process types and associated start commands for the Droplet
	ProcessTypes []DropletProcessType `json:"processTypes"`

	// Specifies the exposed ports for the application
	Ports []int32 `json:"ports,omitempty"`
}

type DropletProcessType struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Default bool   `json:"default"`
}

type Image struct {
	Reference      string `json:"reference"`
	PullSecretName string `json:"pullSecretName"`
}

// DropletStatus defines the observed state of Droplet
type DropletStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// TODO: Open question: Should this be flexible and use the "latestImage" duck type to
	// allow for easier handling of stack updates in the background or should it be closer
	// to the original design of the CF Droplet and only refer to a static image
	ImageRef KpackImageReference `json:"imageRef,omitempty"`

	// Describes Docker metadata including ports the container exposes
	LifecycleData DockerLifecycleData `json:"lifecycleData,omitempty"`

	// Describes the conditions of the Droplet
	Conditions []metav1.Condition `json:"conditions"`
}

type DockerLifecycleData struct {
	// TODO: Decide if we need this
	// Marshalled blob of JSON goo
	ExecutionMetadata string `json:"executionMetadata"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Droplet is the Schema for the droplets API
type Droplet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DropletSpec   `json:"spec,omitempty"`
	Status DropletStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DropletList contains a list of Droplet
type DropletList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Droplet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Droplet{}, &DropletList{})
}
