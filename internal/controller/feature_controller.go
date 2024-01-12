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
	"reflect"
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

	helmv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	bananav1alpha1 "github.com/middlewaregruppen/banana-operator/api/v1alpha1"
)

const (
	HelmChartStatusProgressing = "Progressing"
	HelmChartStatusReady       = "Ready"
	HelmChartStatusDegraded    = "Degraded"
	HelmChartStatusUnknown     = "Unknown"
)

// FeatureReconciler reconciles a Feature object
type FeatureReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=features,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=features/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=features/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Feature object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *FeatureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	// Fetch FeatureSet - This ensures that the cluster has resources of type Feature.
	// Stops reconciliation if not found, for example if the CRD's has not been applied
	feat := &bananav1alpha1.Feature{}
	if err := r.Get(ctx, req.NamespacedName, feat); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var err error

	// Add finalizers that will be handled later during delete events
	if !controllerutil.ContainsFinalizer(feat, featureSetFinalizers) {
		if ok := controllerutil.AddFinalizer(feat, featureSetFinalizers); !ok {
			return ctrl.Result{Requeue: true}, nil
		}
		if err = r.Update(ctx, feat); err != nil {
			return ctrl.Result{}, err
		}
	}

	defer func() {
		// Update resource
		if dErr := r.Status().Update(ctx, feat); dErr != nil {
			l.Error(err, "Failed updating status")
			err = dErr
		}
	}()

	// Run finalizers if resource is marked as deleted
	if !feat.ObjectMeta.DeletionTimestamp.IsZero() {
		l.Info("Deleting")
		return ctrl.Result{}, r.finalize(ctx, feat)
	}

	// Loop through intermediate features and check if Feature exists, if not create a new one
	err = r.ensureHelmChart(ctx, feat)
	if err != nil {
		err = fmt.Errorf("could not reconcile HelmChart: %w", err)
		readyCondition := metav1.Condition{
			Status:             metav1.ConditionFalse,
			Reason:             ReasonReconciliationFailed,
			Message:            err.Error(),
			Type:               TypeFeatureSetAvailable,
			ObservedGeneration: feat.GetGeneration(),
		}
		meta.SetStatusCondition(&feat.Status.Conditions, readyCondition)
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
	}

	r.setReadyStatus(ctx, feat)
	l.Info("successfully reconciled feature")

	return ctrl.Result{}, nil
}

func (r *FeatureReconciler) setReadyStatus(ctx context.Context, feat *bananav1alpha1.Feature) {
	l := log.FromContext(ctx)
	//if feat.Status.NumFeaturesDesired == feat.Status.NumFeaturesReady {
	l.Info("all HelmCharts are in ready state")
	readyCondition := metav1.Condition{
		Status:             metav1.ConditionTrue,
		Reason:             ReasonReconciliationSucceeded,
		Message:            "All HelmCharts are in ready state",
		Type:               TypeFeatureSetAvailable,
		ObservedGeneration: feat.GetGeneration(),
	}
	meta.SetStatusCondition(&feat.Status.Conditions, readyCondition)
	//}
}

func (r *FeatureReconciler) ensureHelmChart(ctx context.Context, feature *bananav1alpha1.Feature) error {

	l := log.FromContext(ctx)
	currentChart, err := r.getHelmChart(ctx, types.NamespacedName{Name: feature.Name, Namespace: feature.Namespace})
	if err != nil {
		return err
	}
	if currentChart == nil {
		l.Info("current HelmChart is not present, creating a new one", "name", feature.Name)
		feature.Status.HelmChartStatus = HelmChartStatusProgressing
		newChart := &helmv1.HelmChart{
			TypeMeta: metav1.TypeMeta{
				Kind:       "HelmChart",
				APIVersion: "helm.cattle.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      feature.Name,
				Namespace: feature.Namespace,
			},
			Spec: feature.Spec.Helm,
		}
		if err := controllerutil.SetControllerReference(feature, newChart, r.Scheme); err != nil {
			return err
		}
		err := applyRuntimeObject(ctx, types.NamespacedName{Name: feature.Name, Namespace: feature.Namespace}, newChart, r.Client)
		if err != nil {
			feature.Status.HelmChartStatus = HelmChartStatusDegraded
			return err
		}

		return nil
	}

	if r.needsUpdate(feature, currentChart) {
		l.Info("chart needs updating", "name", feature.Name)
		return r.Update(ctx, currentChart)
	}

	job, _ := r.getHelmChartJob(ctx, types.NamespacedName{Name: currentChart.Status.JobName, Namespace: currentChart.ObjectMeta.Namespace})
	if job != nil {
		condition := metav1.ConditionUnknown
		for _, c := range job.Status.Conditions {
			l.Info("Condition is", "name", c)
			if c.Status == corev1.ConditionTrue {
				condition = metav1.ConditionStatus(c.Type)
				l.Info("Condition is", "name", condition)
			}
		}
		feature.Status.HelmChartStatus = string(condition)
	}

	return nil
}

func (r *FeatureReconciler) needsUpdate(feature *bananav1alpha1.Feature, chart *helmv1.HelmChart) bool {
	if !reflect.DeepEqual(feature.Spec.Helm, chart.Spec) {
		return true
	}
	return false
}

func (r *FeatureReconciler) getHelmChart(ctx context.Context, name types.NamespacedName) (*helmv1.HelmChart, error) {
	l := log.FromContext(ctx)
	l.Info("Getting helm chart")
	chart := &helmv1.HelmChart{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmChart",
			APIVersion: "helm.cattle.io/v1",
		},
	}
	err := r.Get(ctx, name, chart)
	if err != nil {
		if !errors.IsNotFound(err) {
			return chart, nil
		}
		return nil, nil
	}
	return chart, nil
}

func (r *FeatureReconciler) getHelmChartJob(ctx context.Context, name types.NamespacedName) (*batchv1.Job, error) {
	l := log.FromContext(ctx)
	l.Info("Getting job for helm chart")
	job := &batchv1.Job{}
	err := r.Get(ctx, name, job)
	if err != nil {
		if !errors.IsNotFound(err) {
			return job, nil
		}
		return nil, nil
	}
	return job, nil
}

func (r *FeatureReconciler) finalize(ctx context.Context, feat *bananav1alpha1.Feature) error {
	// Perform finalizers before deleting resource from cluster
	if controllerutil.ContainsFinalizer(feat, featureSetFinalizers) {

		// TODO: run finalizers here. Always delete resources that belong to this CRD before proceeding further
		// Delete all features managed by this featureset
		chart := &helmv1.HelmChart{}
		err := r.Get(ctx, types.NamespacedName{Name: feat.Name, Namespace: feat.Namespace}, chart)
		if err != nil {
			return err
		}

		err = r.Delete(ctx, chart)
		if err != nil {
			return err
		}

		controllerutil.RemoveFinalizer(feat, featureSetFinalizers)
		return r.Update(ctx, feat)

	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FeatureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bananav1alpha1.Feature{}).
		Owns(&helmv1.HelmChart{}).
		Complete(r)
}
