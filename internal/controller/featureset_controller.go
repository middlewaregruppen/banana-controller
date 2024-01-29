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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	bananav1alpha1 "github.com/middlewaregruppen/banana-controller/api/v1alpha1"
)

// Definitions to manage status conditions
const (
	// TypeFeatureSetAvailable represents the status of the HelmChart reconciliation
	TypeFeatureSetAvailable = "Available"

	// ReasonReconciliationFailed
	ReasonReconciliationFailed = "ReconciliationFailed"

	// ReasonReconciliationSucceeded
	ReasonReconciliationSucceeded = "ReconciliationSucceeded"

	featureSetFinalizers = "banana.mdlwr.com/finalizer"
)

// FeatureSetReconciler reconciles a FeatureSet object
type FeatureSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=featseturesets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=featseturesets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=featseturesets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the FeatureSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *FeatureSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	// Fetch FeatureSet - This ensures that the cluster has resources of type Feature.
	// Stops reconciliation if not found, for example if the CRD's has not been applied
	featset := &bananav1alpha1.FeatureSet{}
	if err := r.Get(ctx, req.NamespacedName, featset); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var err error

	// Add finalizers that will be handled later during delete events
	if !controllerutil.ContainsFinalizer(featset, featureSetFinalizers) {
		// if ok := controllerutil.AddFinalizer(featset, featureSetFinalizers); !ok {
		// 	return ctrl.Result{Requeue: true}, nil
		// }
		controllerutil.AddFinalizer(featset, featureSetFinalizers)
		if err = r.Update(ctx, featset); err != nil {
			return ctrl.Result{}, err
		}
	}

	defer func() {
		// Update resource
		if dErr := r.Status().Update(ctx, featset); dErr != nil {
			l.Error(err, "Failed updating status")
			err = dErr
		}
		l.Info("update FeatureSet status field")
	}()

	// Run finalizers if resource is marked as deleted
	if !featset.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.finalize(ctx, featset)
	}

	// Loop through intermediate features and check if Feature exists, if not create a new one
	var lErr error
	for _, interfeat := range featset.Spec.Features {
		lErr = r.ensureFeatures(ctx, r.decorateFeatureWith(featset, &interfeat), featset)
		if lErr != nil {
			lErr = fmt.Errorf("could not reconcile Feature: %w", err)
			readyCondition := metav1.Condition{
				Status:             metav1.ConditionFalse,
				Reason:             ReasonReconciliationFailed,
				Message:            err.Error(),
				Type:               TypeFeatureSetAvailable,
				ObservedGeneration: featset.GetGeneration(),
			}
			meta.SetStatusCondition(&featset.Status.Conditions, readyCondition)
			err = lErr
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
		}
	}

	r.setReadyStatys(ctx, featset)
	l.Info("successfully reconciled featureset")

	return ctrl.Result{}, nil
}

func (r *FeatureSetReconciler) setReadyStatys(ctx context.Context, featset *bananav1alpha1.FeatureSet) {
	l := log.FromContext(ctx)
	if featset.Status.NumFeaturesDesired == featset.Status.NumFeaturesReady {
		l.Info("all features are in ready state")
		readyCondition := metav1.Condition{
			Status:             metav1.ConditionTrue,
			Reason:             ReasonReconciliationSucceeded,
			Message:            "All features are in ready state",
			Type:               TypeFeatureSetAvailable,
			ObservedGeneration: featset.GetGeneration(),
		}
		meta.SetStatusCondition(&featset.Status.Conditions, readyCondition)
	}
}

func (r *FeatureSetReconciler) ensureFeatures(ctx context.Context, feature *bananav1alpha1.Feature, featset *bananav1alpha1.FeatureSet) error {

	l := log.FromContext(ctx)
	featset.Status.NumFeaturesDesired = len(featset.Spec.Features)

	currentFeat, err := r.getFeature(ctx, types.NamespacedName{Name: feature.Name, Namespace: feature.Namespace})
	if err != nil {
		return err
	}
	if currentFeat == nil {
		l.Info("current feature is not present, creating a new one", "name", feature.Name)
		if err := controllerutil.SetControllerReference(featset, feature, r.Scheme); err != nil {
			return err
		}
		err := applyRuntimeObject(ctx, types.NamespacedName{Name: feature.Name, Namespace: feature.Namespace}, feature, r.Client)
		if err != nil {
			return err
		}
		featset.Status.NumFeaturesReady++
		return nil
	}

	if needsUpdate(feature, currentFeat) {
		l.Info("feature needs updating", "name", feature.Name)
		//currentFeat.Spec.Helm = feature.Spec.Helm
		return r.Update(ctx, currentFeat)
	}

	return nil
}

func (r *FeatureSetReconciler) getFeature(ctx context.Context, name types.NamespacedName) (*bananav1alpha1.Feature, error) {
	feat := &bananav1alpha1.Feature{}
	err := r.Get(ctx, types.NamespacedName{Name: name.Name, Namespace: name.Namespace}, feat)
	if err != nil {
		if !errors.IsNotFound(err) {
			return feat, nil
		}
		return nil, nil
	}
	return feat, nil
}

func (r *FeatureSetReconciler) finalize(ctx context.Context, featset *bananav1alpha1.FeatureSet) error {
	// Perform finalizers before deleting resource from cluster
	if controllerutil.ContainsFinalizer(featset, featureSetFinalizers) {

		// TODO: run finalizers here. Always delete resources that belong to this CRD before proceeding further
		// Delete all features managed by this featureset
		var dErr error
		for _, interfeat := range featset.Spec.Features {
			feat, err := r.getFeature(ctx, types.NamespacedName{Name: interfeat.Name, Namespace: featset.Namespace})
			if err != nil {
				dErr = err
			}
			if feat != nil {
				err = r.Delete(ctx, feat)
				if err != nil {
					dErr = err
				}
				featset.Status.NumFeaturesReady--
			}
		}
		if dErr != nil {
			return dErr
		}

		controllerutil.RemoveFinalizer(featset, featureSetFinalizers)
		return r.Update(ctx, featset)

	}
	return nil
}

func applyRuntimeObject(ctx context.Context, key client.ObjectKey, obj client.Object, c client.Client) error {
	getObj := obj
	switch err := c.Get(ctx, key, getObj); {
	case errors.IsNotFound(err):
		return c.Create(ctx, obj)
	case err == nil:
		return c.Update(ctx, obj)
	default:
		return err
	}
}

func needsUpdate(generated, current *bananav1alpha1.Feature) bool {
	// if !reflect.DeepEqual(generated.Spec.Helm, current.Spec.Helm) {
	// 	return true
	// }
	return false
}

func (r *FeatureSetReconciler) decorateFeatureWith(featureset *bananav1alpha1.FeatureSet, intermediate *bananav1alpha1.IntermediateFeature) *bananav1alpha1.Feature {
	return &bananav1alpha1.Feature{
		ObjectMeta: metav1.ObjectMeta{
			Name:      intermediate.Name,
			Namespace: featureset.Namespace,
		},
		Spec: bananav1alpha1.FeatureSpec{
			//Helm: intermediate.Helm,
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FeatureSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bananav1alpha1.FeatureSet{}).
		Owns(&bananav1alpha1.Feature{}).
		Complete(r)
}
