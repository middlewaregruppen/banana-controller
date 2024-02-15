package controller

import (
	"context"

	"github.com/go-logr/logr"
	bananav1alpha1 "github.com/middlewaregruppen/banana-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Definitions to manage status conditions
const (
	// TypeFeatureSetAvailable represents the status of the HelmChart reconciliation
	TypeFeatureSetAvailable = "Available"

	// ReasonReconciliationFailed
	ReasonReconciliationFailed = "ReconciliationFailed"

	// ReasonReconciliationSucceeded
	ReasonReconciliationSucceeded = "ReconciliationSucceeded"

	// Finalizers
	featureSetFinalizers      = "finalizer.banana.mdlwr.com/featureset"
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

func needsUpdate(generated, current *bananav1alpha1.Feature) bool {
	// if !reflect.DeepEqual(generated.Spec.Helm, current.Spec.Helm) {
	// 	return true
	// }
	return false
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
