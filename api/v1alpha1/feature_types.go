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

package v1alpha1

import (
	//argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	//argo "github.com/argoproj/argo-cd/v2"
	helmv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//ktypes "sigs.k8s.io/kustomize/api/types"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FeatureSpec defines the desired state of Feature
type FeatureSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name is the name of this feature
	Helm helmv1.HelmChartSpec `json:"helm,omitempty"`
	//ArgoCD argov1alpha1.ApplicationSpec `json:"argocd,omitempty"`
	//Kustomize ktypes.Kustomization `json:"kustomization,omitempty"`
}

// FeatureStatus defines the observed state of Feature
type FeatureStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions      []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	HelmChartStatus string             `json:"helmChartStatus,omitempty"`
}

//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=features,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=features/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=features/finalizers,verbs=update
//+kubebuilder:rbac:groups=helm.cattle.io,resources=helmcharts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=helm.cattle.io,resources=helmcharts/status,verbs=get

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.helmChartStatus"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Feature is the Schema for the features API
type Feature struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FeatureSpec   `json:"spec,omitempty"`
	Status FeatureStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FeatureList contains a list of Feature
type FeatureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Feature `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Feature{}, &FeatureList{})
}
