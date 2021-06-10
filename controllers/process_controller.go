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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cfappsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// ProcessReconciler reconciles a Process object
type ProcessReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=processes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=processes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=processes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Process object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ProcessReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// fetch Process
	process := new(cfappsv1alpha1.Process)
	if err := r.Get(ctx, req.NamespacedName, process); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Process no longer exists")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// fetch the Droplet to get the imageRef
	droplet := new(cfappsv1alpha1.Droplet)
	if err := r.Get(ctx, types.NamespacedName{Name: process.Spec.DropletRef.Name, Namespace: req.Namespace}, droplet); err != nil {
		// deal with this later - maybe requeue
	}
	// may need to verify that the droplet imageRef is in the status

	memory := *resource.NewScaledQuantity(process.Spec.MemoryMB, resource.Mega)
	ephemeralStorage := *resource.NewScaledQuantity(process.Spec.DiskQuotaMB, resource.Mega)

	// build the Deployment that we want
	desiredDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"apps.cloudfoundry.org/appGuid":     process.Spec.AppRef.Name,
				"apps.cloudfoundry.org/processGuid": process.Name,
				"apps.cloudfoundry.org/processType": process.Spec.ProcessType,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: cfappsv1alpha1.SchemeBuilder.GroupVersion.String(),
					Kind:       process.Kind,
					Name:       process.Name,
					UID:        process.UID,
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: toInt32Ptr(int32(process.Spec.Instances)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"apps.cloudfoundry.org/processGuid": process.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"apps.cloudfoundry.org/appGuid":     process.Spec.AppRef.Name,
						"apps.cloudfoundry.org/processGuid": process.Name,
						"apps.cloudfoundry.org/processType": process.Spec.ProcessType,
					},
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: droplet.Status.Image.PullSecretName},
					},
					Containers: []corev1.Container{
						{
							Name:    process.Spec.ProcessType,
							Image:   droplet.Status.Image.Reference,
							Command: commandForProcess(process),
							Ports:   portsForProcess(process),
							EnvFrom: []corev1.EnvFromSource{
								{SecretRef: &corev1.SecretEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: process.Spec.EnvSecretName,
									},
								}},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory:           memory,
									corev1.ResourceEphemeralStorage: ephemeralStorage,
								},
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: memory,
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: nil,
									},
									HTTPGet: &corev1.HTTPGetAction{
										Path: "",
										Port: intstr.IntOrString{
											Type:   0,
											IntVal: 0,
											StrVal: "",
										},
										Host:        "",
										Scheme:      "",
										HTTPHeaders: nil,
									},
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											Type:   0,
											IntVal: 0,
											StrVal: "",
										},
										Host: "",
									},
								},
								InitialDelaySeconds: 0,
								TimeoutSeconds:      0,
								PeriodSeconds:       0,
								SuccessThreshold:    0,
								FailureThreshold:    0,
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: nil,
									},
									HTTPGet: &corev1.HTTPGetAction{
										Path: "",
										Port: intstr.IntOrString{
											Type:   0,
											IntVal: 0,
											StrVal: "",
										},
										Host:        "",
										Scheme:      "",
										HTTPHeaders: nil,
									},
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											Type:   0,
											IntVal: 0,
											StrVal: "",
										},
										Host: "",
									},
								},
								InitialDelaySeconds: 0,
								TimeoutSeconds:      0,
								PeriodSeconds:       0,
								SuccessThreshold:    0,
								FailureThreshold:    0,
							},
							StartupProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: nil,
									},
									HTTPGet: &corev1.HTTPGetAction{
										Path: "",
										Port: intstr.IntOrString{
											Type:   0,
											IntVal: 0,
											StrVal: "",
										},
										Host:        "",
										Scheme:      "",
										HTTPHeaders: nil,
									},
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											Type:   0,
											IntVal: 0,
											StrVal: "",
										},
										Host: "",
									},
								},
								InitialDelaySeconds: 0,
								TimeoutSeconds:      0,
								PeriodSeconds:       0,
								SuccessThreshold:    0,
								FailureThreshold:    0,
							},
							Lifecycle: &corev1.Lifecycle{
								PostStart: &corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: nil,
									},
									HTTPGet: &corev1.HTTPGetAction{
										Path: "",
										Port: intstr.IntOrString{
											Type:   0,
											IntVal: 0,
											StrVal: "",
										},
										Host:        "",
										Scheme:      "",
										HTTPHeaders: nil,
									},
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											Type:   0,
											IntVal: 0,
											StrVal: "",
										},
										Host: "",
									},
								},
								PreStop: &corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: nil,
									},
									HTTPGet: &corev1.HTTPGetAction{
										Path: "",
										Port: intstr.IntOrString{
											Type:   0,
											IntVal: 0,
											StrVal: "",
										},
										Host:        "",
										Scheme:      "",
										HTTPHeaders: nil,
									},
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											Type:   0,
											IntVal: 0,
											StrVal: "",
										},
										Host: "",
									},
								},
							},
							TerminationMessagePath:   "",
							TerminationMessagePolicy: "",
							ImagePullPolicy:          "",
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add:  nil,
									Drop: nil,
								},
								Privileged: nil,
								SELinuxOptions: &corev1.SELinuxOptions{
									User:  "",
									Role:  "",
									Type:  "",
									Level: "",
								},
								WindowsOptions: &corev1.WindowsSecurityContextOptions{
									GMSACredentialSpecName: nil,
									GMSACredentialSpec:     nil,
									RunAsUserName:          nil,
								},
								RunAsUser:                nil,
								RunAsGroup:               nil,
								RunAsNonRoot:             nil,
								ReadOnlyRootFilesystem:   nil,
								AllowPrivilegeEscalation: nil,
								ProcMount:                nil,
								SeccompProfile: &corev1.SeccompProfile{
									Type:             "",
									LocalhostProfile: nil,
								},
							},
							Stdin:     false,
							StdinOnce: false,
							TTY:       false,
						},
					},
				},
			},
		},
	}
	// fetch the existing Deployment if one exists. Make a new one otherwise (in memory only?)
	// set all fields on the Deployment
	// apply the Deployment
	// https://github.com/cloudfoundry/cf-k8s-networking/blob/d1ee303823b0bbaa1013a3221bf689207b6f1aca/routecontroller/controllers/networking/route_controller.go#L122
	return ctrl.Result{}, nil
}

func portsForProcess(process *cfappsv1alpha1.Process) []corev1.ContainerPort {
	var ports []corev1.ContainerPort
	for _, port := range process.Spec.Ports {
		ports = append(ports, corev1.ContainerPort{
			ContainerPort: int32(port),
		})
	}
	return ports
}

func commandForProcess(process *cfappsv1alpha1.Process) []string {
	if process.Spec.Command == "" {
		return nil
	} else if process.Spec.LifecycleType == "kpack" {
		return []string{"/cnb/lifecycle/launcher", process.Spec.Command}
	} else {
		return []string{"/bin/sh", "-c", process.Spec.Command}
	}
}

func toInt32Ptr(i int32) *int32 {
	return &i
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProcessReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: should we filter events here?
	return ctrl.NewControllerManagedBy(mgr).
		For(&cfappsv1alpha1.Process{}).
		Complete(r)
}
