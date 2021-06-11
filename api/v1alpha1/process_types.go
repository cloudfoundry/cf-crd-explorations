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

// ProcessSpec defines the desired state of Process
type ProcessSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Specifies the App that owns this process
	AppRef ApplicationReference `json:"appRef"`

	// Specifies the name of the process in the App
	ProcessType string `json:"processType"`

	// Specifies the Command(k8s) ENTRYPOINT(Docker) of the Process
	Command string `json:"command"`

	// Specifies the current state of the process
	// Valid values are:
	// "STARTED": App is started
	// "STOPPED": App is stopped
	State DesiredState `json:"state"`

	// Specifies the Liveness Probe (k8s) details of the Process
	HealthCheck HealthCheck `json:"healthCheck"`

	// Specifies the number of Process replicas to deploy
	Instances int64 `json:"instances"`

	// Specifies the Process memory limit
	MemoryMB int64 `json:"memoryMB"`

	// Specifies the Process disk limit
	DiskQuotaMB int64 `json:"diskQuotaMB"`

	// Specifies the Process ports to expose
	Ports []int64 `json:"ports"`

	// Specifies the sidecars to be run alongside the Process
	// TODO: Should this be its own CRD?, essentially lives at AppManifest and Process level simultaneously
	Sidecars []ProcessSidecar `json:"sidecars"`
}

type HealthCheck struct {
	// Specifies the type of Health Check the App process will use
	// Valid values are:
	// "http": http health check
	// "port": TCP health check
	// "process" (default): checks if process for start command is still alive
	Type HealthCheckType `json:"type"`

	// Specifies the input parameters for the liveness probe/health check in kubernetes
	Data HealthCheckData `json:"data"`
}

// HealthCheckData used to pass through input parameters to liveness probe
type HealthCheckData struct {
	// HTTPEndpoint is only used by an "http" liveness probe
	// +optional
	HTTPEndpoint string `json:"httpEndpoint,omitempty"`

	InvocationTimeoutSeconds int64 `json:"invocationTimeoutSeconds"`
	TimeoutSeconds           int64 `json:"timeoutSeconds"`
}

// HealthCheckType used to ensure illegal HealthCheckTypes are not passed
// +kubebuilder:validation:Enum=http;port;process
type HealthCheckType string

const (
	HTTPHealthCheckType    = "http"
	PortHealthCheckType    = "port"
	ProcessHealthCheckType = "process"
)

// ProcessSidecar defines sidecars explicitly run with the Process
type ProcessSidecar struct {
	Name string `json:"name"`
	// Command is the K8s Command/ENTRYPOINT
	Command  string `json:"command"`
	MemoryMB int64  `json:"memoryMB"`
}

// ProcessStatus defines the observed state of Process
type ProcessStatus struct {
	Instances  int64       `json:"instances"`
	Conditions []Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Process is the Schema for the processes API
type Process struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProcessSpec   `json:"spec,omitempty"`
	Status ProcessStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ProcessList contains a list of Process
type ProcessList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Process `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Process{}, &ProcessList{})
}
