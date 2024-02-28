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

	bananav1alpha1 "github.com/middlewaregruppen/banana-controller/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/middlewaregruppen/banana-controller/pkg/config"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// FeatureOverrideReconciler reconciles a FeatureOverride object
type FeatureOverrideReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.Config
}

//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=featureoverrides,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=featureoverrides/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=featureoverrides/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the FeatureOverride object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *FeatureOverrideReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	// Fetch Feature - This ensures that the cluster has resources of type Feature.
	// Stops reconciliation if not found, for example if the CRD's has not been applied
	featovr := &bananav1alpha1.FeatureOverride{}
	if err := r.Get(ctx, req.NamespacedName, featovr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizers that will be handled later during delete events
	if !controllerutil.ContainsFinalizer(featovr, featureOverrideFinalizers) {
		if ok := controllerutil.AddFinalizer(featovr, featureOverrideFinalizers); !ok {
			return ctrl.Result{Requeue: true}, nil
		}
		controllerutil.AddFinalizer(featovr, featureOverrideFinalizers)
		if err := r.Update(ctx, featovr); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Iterate over all features that this featureoverride is selecting and then add them to the status field
	featureRefs := []string{}
	features, err := getFeaturesForOverride(ctx, r.Client, types.NamespacedName{Name: featovr.Name})
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, feature := range features {
		fName := types.NamespacedName{Name: feature.Name, Namespace: feature.Namespace}
		featureRefs = append(featureRefs, fName.String())
	}
	featovr.Status.FeatureRefs = featureRefs

	// TODO: Deleted resources can't be updated
	defer func() {
		// Update resource
		if err := r.Status().Update(ctx, featovr); err != nil {
			logError(featovr, "failed updating status", l, err)
		}
	}()

	// Run finalizers if resource is marked as deleted
	if !featovr.ObjectMeta.DeletionTimestamp.IsZero() {
		l.Info("resource is marked for deletion, running finalizers", "name", featovr.Name, "namespace", featovr.Namespace)
		return ctrl.Result{}, r.finalize(ctx, featovr)
	}

	// Trigger reconciliation of all Features targeted by this FeatureOverride
	// for _, feature := range features {
	// 	l.Info("Updating features", "num", len(features))
	// 	if err := r.Status().Update(ctx, feature); err != nil {
	// 		return ctrl.Result{}, err
	// 	}
	// }

	l.Info("successfully reconciled featureoverride")
	featuresTotalCounter.WithLabelValues(featovr.GetName(), "reconciled").Inc()

	return ctrl.Result{}, nil
}

func (r *FeatureOverrideReconciler) finalize(ctx context.Context, feat *bananav1alpha1.FeatureOverride) error {
	// Perform finalizers before deleting resource from cluster
	if controllerutil.ContainsFinalizer(feat, featureOverrideFinalizers) {

		// TODO: Cleanup Argo APP by removing the Helm values that this featureoverride initially added
		//finalizeFeatureValues(ctx, r.Client, feat)

		controllerutil.RemoveFinalizer(feat, featureOverrideFinalizers)
		return r.Update(ctx, feat)

	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FeatureOverrideReconciler) SetupWithManager(mgr ctrl.Manager, cfg *config.Config) error {
	r.Config = cfg
	return ctrl.NewControllerManagedBy(mgr).
		For(&bananav1alpha1.FeatureOverride{}).
		Complete(r)
}
