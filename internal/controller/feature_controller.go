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

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	bananav1alpha1 "github.com/middlewaregruppen/banana-controller/api/v1alpha1"
)

const (
	ArgoApplicationStatusProgressing = "Progressing"
	ArgoApplicationStatusReady       = "Ready"
	ArgoApplicationStatusDegraded    = "Degraded"
	ArgoApplicationStatusUnknown     = "Unknown"
)

var (
	featuresTotalCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "banana_features_total",
			Help: "Number of features processed",
		},
		[]string{"feature", "reason"},
	)
	featuresErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "banana_features_error_total",
			Help: "Number of failed features",
		},
		[]string{"feature", "reason"},
	)
)

// FeatureReconciler reconciles a Feature object
type FeatureReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func init() {
	metrics.Registry.MustRegister(featuresTotalCounter, featuresErrorCounter)
}

func logError(feature *bananav1alpha1.Feature, s string, l logr.Logger, err error) {
	featuresErrorCounter.WithLabelValues(feature.GetName(), err.Error()).Inc()
	l.Error(err, s)
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

	// Add finalizers that will be handled later during delete events
	if !controllerutil.ContainsFinalizer(feat, featureSetFinalizers) {
		if ok := controllerutil.AddFinalizer(feat, featureSetFinalizers); !ok {
			return ctrl.Result{Requeue: true}, nil
		}
		controllerutil.AddFinalizer(feat, featureSetFinalizers)
		if err := r.Update(ctx, feat); err != nil {
			return ctrl.Result{}, err
		}
	}

	defer func() {
		// Update resource
		if err := r.Status().Update(ctx, feat); err != nil {
			//l.Error(err, "Failed updating status")
			logError(feat, "failed updating status", l, err)
		}
	}()

	// Run finalizers if resource is marked as deleted
	if !feat.ObjectMeta.DeletionTimestamp.IsZero() {
		l.Info("Deleting")
		return ctrl.Result{}, r.finalize(ctx, feat)
	}

	// Check if Argo Application exists, if not create a new one
	err := r.ensureArgoApp(ctx, feat)
	if err != nil {
		logError(feat, "could not reconcile Application", l, err)
		err = fmt.Errorf("could not reconcile Application: %w", err)
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
	featuresTotalCounter.WithLabelValues(feat.GetName(), "reconciled").Inc()

	return ctrl.Result{}, nil
}

func (r *FeatureReconciler) setReadyStatus(ctx context.Context, feat *bananav1alpha1.Feature) {
	l := log.FromContext(ctx)
	l.Info("all Argo Applications are in ready state")
	readyCondition := metav1.Condition{
		Status:             metav1.ConditionTrue,
		Reason:             ReasonReconciliationSucceeded,
		Message:            "All Applications are in ready state",
		Type:               TypeFeatureSetAvailable,
		ObservedGeneration: feat.GetGeneration(),
	}
	meta.SetStatusCondition(&feat.Status.Conditions, readyCondition)
}

func (r *FeatureReconciler) ensureArgoApp(ctx context.Context, feature *bananav1alpha1.Feature) error {

	l := log.FromContext(ctx)

	// Get the Argo App by it's name. Create an app if nothing is found
	currentApp, err := r.getArgoApp(ctx, types.NamespacedName{Name: feature.Name, Namespace: feature.Namespace})
	if err != nil {
		return err
	}
	if currentApp == nil {
		l.Info("current Application is not present, creating a new one", "name", feature.Name)
		feature.Status.SyncStatus = ArgoApplicationStatusProgressing
		newApp := r.constructArgoApp(feature)
		if err := controllerutil.SetControllerReference(feature, newApp, r.Scheme); err != nil {
			return err
		}
		err := applyRuntimeObject(ctx, types.NamespacedName{Name: feature.Name, Namespace: feature.Namespace}, newApp, r.Client)
		if err != nil {
			feature.Status.SyncStatus = ArgoApplicationStatusDegraded
			return err
		}

		return nil
	}

	// Update statuses
	feature.Status.SyncStatus = string(currentApp.Status.Sync.Status)
	feature.Status.HealthStatus = string(currentApp.Status.Health.Status)
	feature.Status.Images = currentApp.Status.Summary.Images
	feature.Status.URLs = currentApp.Status.Summary.ExternalURLs

	// Check if the Argo App needs updating by comparing their Specs
	if r.needsUpdate(r.constructArgoApp(feature), currentApp) {
		l.Info("application needs updating", "name", feature.Name)
		currentApp.Spec = r.constructArgoApp(feature).Spec
		return r.Update(ctx, currentApp)
	}

	return nil
}

func (r *FeatureReconciler) constructArgoApp(feature *bananav1alpha1.Feature) *argov1alpha1.Application {
	proj := feature.Spec.Project
	if len(feature.Spec.Project) == 0 {
		proj = "default"
	}
	return &argov1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      feature.Name,
			Namespace: feature.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/instance": feature.GetObjectMeta().GetName(),
			},
		},
		Spec: argov1alpha1.ApplicationSpec{
			Project: proj,
			Destination: argov1alpha1.ApplicationDestination{
				Namespace: feature.Spec.Namespace,
				Server:    "https://kubernetes.default.svc",
			},
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        feature.Spec.Repo,
				Path:           feature.Spec.Path,
				Chart:          feature.Spec.Name,
				TargetRevision: feature.Spec.Revision,
				Helm: &argov1alpha1.ApplicationSourceHelm{
					Parameters: feature.Spec.Values,
				},
			},
			SyncPolicy: &feature.Spec.SyncPolicy,
		},
	}
}

func (r *FeatureReconciler) needsUpdate(old, new *argov1alpha1.Application) bool {
	return !reflect.DeepEqual(old.Spec, new.Spec)
}

func (r *FeatureReconciler) getArgoApp(ctx context.Context, name types.NamespacedName) (*argov1alpha1.Application, error) {
	l := log.FromContext(ctx)
	l.Info("Getting Application")
	argoapp := &argov1alpha1.Application{}
	err := r.Get(ctx, name, argoapp)
	if err != nil {
		if !errors.IsNotFound(err) {
			return argoapp, nil
		}
		return nil, nil
	}
	return argoapp, nil
}

func (r *FeatureReconciler) finalize(ctx context.Context, feat *bananav1alpha1.Feature) error {
	// Perform finalizers before deleting resource from cluster
	if controllerutil.ContainsFinalizer(feat, featureSetFinalizers) {

		// TODO: run finalizers here. Always delete resources that belong to this CRD before proceeding further
		// Delete all features managed by this featureset
		argoapp := &argov1alpha1.Application{}
		err := r.Get(ctx, types.NamespacedName{Name: feat.Name, Namespace: feat.Namespace}, argoapp)
		if err != nil {
			return err
		}

		err = r.Delete(ctx, argoapp)
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
		Owns(&argov1alpha1.Application{}).
		Complete(r)
}
