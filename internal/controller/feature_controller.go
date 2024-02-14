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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/teris-io/shortid"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/middlewaregruppen/banana-controller/pkg/config"

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
	Config config.Config
}

func init() {
	metrics.Registry.MustRegister(featuresTotalCounter, featuresErrorCounter)
}

type bananaTraceIdKey string

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
	uid, _ := shortid.Generate()
	k := bananaTraceIdKey("banana-trace-id")

	ctx = context.WithValue(ctx, k, uid)
	l := log.FromContext(ctx).WithName(uid)

	// Fetch Feature - This ensures that the cluster has resources of type Feature.
	// Stops reconciliation if not found, for example if the CRD's has not been applied
	feat := &bananav1alpha1.Feature{}
	if err := r.Get(ctx, req.NamespacedName, feat); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizers that will be handled later during delete events.
	// Only add finalizers if the resource doesn't contain a finalizer and it's not marked for delettion
	if !controllerutil.ContainsFinalizer(feat, featureFinalizers) && feat.ObjectMeta.DeletionTimestamp.IsZero() {
		if ok := controllerutil.AddFinalizer(feat, featureFinalizers); !ok {
			return ctrl.Result{Requeue: true}, nil
		}
		controllerutil.AddFinalizer(feat, featureFinalizers)
		if err := r.Update(ctx, feat); err != nil {
			return ctrl.Result{}, err
		}
	}

	// TODO: Deleted resources can't be updated
	defer func() {
		// Update resource
		if err := r.Status().Update(ctx, feat); err != nil {
			logError(feat, "failed updating status", l, err)
		}
	}()

	// Run finalizers if resource is marked as deleted
	if !feat.ObjectMeta.DeletionTimestamp.IsZero() {
		l.Info("resource is marked for deletion, running finalizers", "name", feat.Name, "namespace", feat.Namespace)
		return ctrl.Result{}, r.finalize(ctx, feat)
	}

	// Check if Argo Application exists, if not create a new one
	if err := r.ensureArgoApp(ctx, r.Client, feat); err != nil && !errors.IsNotFound(err) {
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
	readyCondition := metav1.Condition{
		Status:             metav1.ConditionTrue,
		Reason:             ReasonReconciliationSucceeded,
		Message:            "All Applications are in ready state",
		Type:               TypeFeatureSetAvailable,
		ObservedGeneration: feat.GetGeneration(),
	}
	meta.SetStatusCondition(&feat.Status.Conditions, readyCondition)
}

func (r *FeatureReconciler) ensureFeatureOverrides(ctx context.Context, c client.Client, feature *bananav1alpha1.Feature, p *Patcher) error {

	overrides := &bananav1alpha1.FeatureOverrideList{}
	err := c.List(ctx, overrides)
	if err != nil {
		return err
	}

	if overrides.Items != nil {
		for _, override := range overrides.Items {
			o := override
			selector := labels.SelectorFromSet(o.Spec.FeatureSelector.MatchLabels)
			l := labels.Set{}
			l = feature.Labels
			if selector.Matches(l) {
				p.Add(override.Spec.Values)
			}
		}
	}
	return nil
}

func (r *FeatureReconciler) ensureArgoApp(ctx context.Context, c client.Client, feature *bananav1alpha1.Feature) error {
	k := bananaTraceIdKey("banana-trace-id")
	l := log.FromContext(ctx).WithName(ctx.Value(k).(string))

	// Create a patcher for this feature
	p := NewPatcherFor(feature.Spec.Values)

	// Get the Argo App by it's name. Create an app if nothing is found
	l.Info("will try to get Application", "name", feature.Name, "Namespace", feature.Namespace)
	currentApp, err := r.getArgoApp(ctx, types.NamespacedName{Name: feature.Name, Namespace: feature.Namespace})
	if err != nil {
		return err
	}

	if currentApp == nil {
		l.Info("current Application not found. It probably doesn't exist, creating a new one", "name", feature.Name, "namespace", feature.Namespace)
		feature.Status.SyncStatus = ArgoApplicationStatusProgressing
		newApp := r.constructArgoApp(feature, nil)
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

	err = r.ensureFeatureOverrides(ctx, c, feature, p)
	if err != nil {
		return err
	}

	values, err := p.Build()
	if err != nil {
		return err
	}

	// Check if the Argo App needs updating by comparing their Specs
	newApp := r.constructArgoApp(feature, values)
	if r.needsUpdate(currentApp, newApp) {
		l.Info("application needs updating", "name", feature.Name, "namespace", feature.Namespace)
		currentApp.Spec = newApp.Spec
		return r.Update(ctx, currentApp)
	}

	// Update statuses
	updateStatus(feature, currentApp)

	return nil
}

func updateStatus(feature *bananav1alpha1.Feature, app *argov1alpha1.Application) {
	// Update statuses
	feature.Status.SyncStatus = string(app.Status.Sync.Status)
	feature.Status.HealthStatus = string(app.Status.Health.Status)
	feature.Status.Images = app.Status.Summary.Images
	feature.Status.URLs = app.Status.Summary.ExternalURLs
}

func (r *FeatureReconciler) constructArgoApp(feature *bananav1alpha1.Feature, b *runtime.RawExtension) *argov1alpha1.Application {
	proj := feature.Spec.Project
	if len(feature.Spec.Project) == 0 {
		proj = "default"
	}

	values := b
	if b == nil {
		values = &runtime.RawExtension{}
	}

	// First item in list is the chart itself
	source := &argov1alpha1.ApplicationSource{
		RepoURL:        feature.Spec.Repo,
		Path:           feature.Spec.Path,
		Chart:          feature.Spec.Name,
		TargetRevision: feature.Spec.Revision,
		Helm: &argov1alpha1.ApplicationSourceHelm{
			ValuesObject: values,
		},
	}

	// Return the constructed multi-source argocd application
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
			Source:     source,
			SyncPolicy: &feature.Spec.SyncPolicy,
		},
	}
}

// Checkout https://pkg.go.dev/github.com/google/go-cmp/cmp
func (r *FeatureReconciler) needsUpdate(old, new *argov1alpha1.Application) bool {

	return !reflect.DeepEqual(old.Spec, new.Spec)
}

func (r *FeatureReconciler) getArgoApp(ctx context.Context, name types.NamespacedName) (*argov1alpha1.Application, error) {
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
	if controllerutil.ContainsFinalizer(feat, featureFinalizers) {

		// TODO: run finalizers here. Always delete resources that belong to this CRD before proceeding further
		// Delete all features managed by this featureset.
		argoapp := &argov1alpha1.Application{}
		if err := r.Get(ctx, types.NamespacedName{Name: feat.Name, Namespace: feat.Namespace}, argoapp); err == nil {
			return r.Delete(ctx, argoapp)
		}

		controllerutil.RemoveFinalizer(feat, featureFinalizers)
		return r.Update(ctx, feat)

	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FeatureReconciler) SetupWithManager(mgr ctrl.Manager, cfg config.Config) error {
	r.Config = cfg
	return ctrl.NewControllerManagedBy(mgr).
		For(&bananav1alpha1.Feature{}).
		Owns(&argov1alpha1.Application{}).
		Complete(r)
}
