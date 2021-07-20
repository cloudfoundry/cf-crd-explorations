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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "cloudfoundry.org/cf-crd-explorations/api/v1alpha1"
)

// PackageReconciler reconciles a Package object
type PackageReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=packages,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=packages/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.cloudfoundry.org,resources=packages/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Package object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *PackageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// your logic here

	return ctrl.Result{}, nil
}

var (
	metadataName = "metadata.name"
)

// SetupWithManager sets up the controller with the Manager.
func (r *PackageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Set up field indexer for for metadata.name
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &appsv1alpha1.Package{}, metadataName, func(rawObj client.Object) []string {
		pkg := rawObj.(*appsv1alpha1.Package)
		return []string{pkg.ObjectMeta.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Package{}).
		Complete(r)
}
