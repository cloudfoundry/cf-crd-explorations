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
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AppManifestSpec defines the desired state of AppManifest
type AppManifestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Name string `json:"name"`

	Buildpacks []string `json:"buildpacks"`

	Env map[string]string `json:"env"`

	Routes []Route `json:"routes"`

	// Why are we using runtime.RawExtension?: https://github.com/kubernetes-sigs/controller-tools/issues/294

	Services []runtime.RawExtension `json:"services"`

	Stack string `json:"stack"`

	Processes []ManifestProcess `json:"processes"`

	Sidecars []Sidecar `json:"sidecars"`
}

type Sidecar struct {
	Name         string   `json:"name"`
	ProcessTypes []string `json:"process_types"`
	Command      string   `json:"command"`
	// TODO: have discussion on input validation like "10M" kubebuilder may have special input type that we can use
	Memory string `json:"memory"`
}

type Route struct {
	Route string `json:"route"`
}

// TODO: all of these fields probably need to be marked as optional json omitempty
// make call after we decide if Manifests CRD is the way
type ManifestProcess struct {
	Type                    string `json:"type"`
	Command                 string `json:"command,omitempty"`
	Memory                  string `json:"memory,omitempty"`
	DiskQuota               string `json:"disk_quota,omitempty"`
	HealthCheckHTTPEndpoint string `json:"health-check-http-endpoint,omitempty"`
	HealthCheckType         string `json:"health-check-type,omitempty"`

	// When it can be opitional need to use *int64
	// TODO: We need to think through how defaulting for omitted values might work in the shim
	Timeout                      *int64 `json:"timeout,omitempty"`
	HealthCheckInvocationTimeout *int64 `json:"health-check-invocation-timeout,omitempty"`
	Instances                    *int64 `json:"instances,omitempty"`
}

// AppManifestStatus defines the observed state of AppManifest
type AppManifestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AppManifest is the Schema for the appmanifests API
type AppManifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppManifestSpec   `json:"spec,omitempty"`
	Status AppManifestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AppManifestList contains a list of AppManifest
type AppManifestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppManifest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppManifest{}, &AppManifestList{})
}
