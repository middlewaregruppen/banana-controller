package controller

import (
	"context"

	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/go-logr/logr"
	bananav1alpha1 "github.com/middlewaregruppen/banana-controller/api/v1alpha1"
	"github.com/middlewaregruppen/banana-controller/pkg/config"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Definitions to manage status conditions
const (
	// TypeFeatureAvailable represents the status of the HelmChart reconciliation
	TypeFeatureAvailable = "Available"

	// ReasonReconciliationFailed
	ReasonReconciliationFailed = "ReconciliationFailed"

	// ReasonReconciliationSucceeded
	ReasonReconciliationSucceeded = "ReconciliationSucceeded"

	// Finalizers
	featureFinalizers         = "finalizer.banana.mdlwr.com/feature"
	featureOverrideFinalizers = "finalizer.banana.mdlwr.com/featureoverride"
)

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

func logError(obj client.Object, s string, l logr.Logger, err error) {
	switch t := obj.(type) {
	case *bananav1alpha1.Feature:
		featuresErrorCounter.WithLabelValues(t.GetName(), err.Error()).Inc()
		l.Error(err, s)
	case *bananav1alpha1.FeatureOverride:
		// TODO: Change to featureOVERRIDE counter
		featuresErrorCounter.WithLabelValues(t.GetName(), err.Error()).Inc()
		l.Error(err, s)
	}
}

// Returns an ArgoCD Application sync policy using the provided Feature. If sync policy == nil
// then an automated sync policy with CreateNamespace=true is returned.
func getArgoSyncPolicy(feature *bananav1alpha1.Feature) *argov1alpha1.SyncPolicy {
	if feature.Spec.SyncPolicy == nil {
		return &argov1alpha1.SyncPolicy{
			Automated:   &argov1alpha1.SyncPolicyAutomated{},
			SyncOptions: []string{"CreateNamespace=true"},
		}
	}
	return feature.Spec.SyncPolicy
}

func getRepoURL(feature *bananav1alpha1.Feature, c *config.Config) string {
	repo := feature.Spec.Repo
	if len(repo) == 0 {
		repo = c.DefaultHelmRepo
	}
	return repo
}
