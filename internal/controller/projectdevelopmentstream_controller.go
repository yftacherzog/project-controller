/*
Copyright 2024.

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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	projctlv1beta1 "github.com/konflux-ci/project-controller/api/v1beta1"
)

// ProjectDevelopmentStreamReconciler reconciles a ProjectDevelopmentStream object
type ProjectDevelopmentStreamReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=projctl.konflux.dev,resources=projectdevelopmentstreams,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=projctl.konflux.dev,resources=projectdevelopmentstreams/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=projctl.konflux.dev,resources=projectdevelopmentstreams/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ProjectDevelopmentStream object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *ProjectDevelopmentStreamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	log := log.FromContext(ctx)

	var pds projctlv1beta1.ProjectDevelopmentStream
	if err := r.Get(ctx, req.NamespacedName, &pds); err != nil {
		log.Error(err, "Unable to fetch ProjectDevelopmentStream")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log = log.WithValues("PDS name", pds.ObjectMeta.Name)
	log.Info("Applying resources from ProjectDevelopmentStream")

	ctrlResult := ctrl.Result{}
	for _, resourceTemplate := range pds.Spec.Resources {
		resource := resourceTemplate.DeepCopy()
		log := log.WithValues(
			"apiVersion", resource.GetAPIVersion(),
			"kind", resource.GetKind(),
			"name", resource.GetName(),
		)
		log.Info("Creating/Updating resource")
		if resource.GetNamespace() != "" && resource.GetNamespace() != pds.GetNamespace() {
			log.Info(
				"Resource namespace set to ProjectDevelopmentStream namespace",
				"PDS namespace", pds.GetNamespace(),
				"resource original namespace", resource.GetNamespace(),
			)
		}
		resource.SetNamespace(pds.GetNamespace())

		var existing unstructured.Unstructured
		existing.SetAPIVersion(resource.GetAPIVersion())
		existing.SetKind(resource.GetKind())
		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(resource), &existing); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Creating new resource")
				if err := r.Client.Create(ctx, resource); err != nil {
					log.Error(err, "Failed to create resource")
				}
			} else {
				log.Error(err, "Failed to read existing resource")
			}
			continue
		}
		update := existing.DeepCopy()
		if m, ok, _ := unstructured.NestedMap(resource.Object, "spec"); ok {
			if err := unstructured.SetNestedMap(update.Object, m, "spec"); err != nil {
				log.Error(err, "Failed to update 'spec' for generated resource")
			}
		}
		if equality.Semantic.DeepEqual(existing.Object, update.Object) {
			log.Info("Resource already up to date")
			continue
		}
		if err := r.Client.Update(ctx, update); err != nil {
			log.Error(err, "Failed to update resource")
			if apierrors.IsConflict(err) {
				ctrlResult = ctrl.Result{Requeue: true}
			}
		}
		log.Info("Resource updated")
	}

	return ctrlResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectDevelopmentStreamReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projctlv1beta1.ProjectDevelopmentStream{}).
		Complete(r)
}