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

// AppSpec defines the desired state of App
type AppSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Name string `json:"name"`

	// Specifies the current state of the app
	// Valid values are:
	// "STARTED": App is started
	// "STOPPED": App is stopped
	DesiredState DesiredState `json:"desiredState"`

	// Specifies the CF Lifecycle type:
	// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#sample-requests
	// Valid values are:
	// "docker": run prebuilt docker image
	// "buildpack": stage the app using kpack
	Type LifecycleType `json:"type,omitempty"`

	// Specifies how to build droplets and run apps
	// container for list of buildpacks and stack to build them
	// for docker this is empty
	Lifecycle Lifecycle `json:"lifecycle,omitempty"`

	// Specifies the k8s secret name with the App credentials and other private info
	EnvSecretName string `json:"envSecretName"`

	// Specifies the Droplet info for the droplet that is currently assigned (active) for the app
	CurrentDropletRef DropletReference `json:"currentDropletRef"`
}

// AppStatus defines the observed state of App
type AppStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// TODO: for each LRP should we propagate some status up if that's useful?

}

type Lifecycle struct {
	// Lifecycle data used to specify details for the Lifecycle
	Data LifecycleData `json:"data"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// App is the Schema for the apps API
// CF API Docs for App:
// https://v3-apidocs.cloudfoundry.org/version/3.101.0/index.html#the-app-object
type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec,omitempty"`
	Status AppStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AppList contains a list of App
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}

func init() {
	SchemeBuilder.Register(&App{}, &AppList{})
}
