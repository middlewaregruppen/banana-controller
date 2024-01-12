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
	helmv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FeatureSetSpec defines the desired state of FeatureSet
type FeatureSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of FeatureSet. Edit featureset_types.go to remove/update
	Features []IntermediateFeature `json:"features,omitempty"`
}

type IntermediateFeature struct {
	Name string               `json:"name,omitempty"`
	Helm helmv1.HelmChartSpec `json:"helm,omitempty"`
}

// FeatureSetStatus defines the observed state of FeatureSet
type FeatureSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// NumFeatures is the number of features managed by this featureset
	NumFeaturesDesired   int `json:"numFeaturesDesired,omitempty"`
	NumFeaturesReady     int `json:"numFeaturesReady,omitempty"`
	NumFeaturesAvailable int `json:"numFeaturesAvailable,omitempty"`
}

//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=featuressets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=featuressets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=featuressets/finalizers,verbs=update
//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=features,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=banana.mdlwr.se,resources=features/status,verbs=get

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Desired",type="integer",JSONPath=".status.numFeaturesDesired"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.numFeaturesReady"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// FeatureSet is the Schema for the featuresets API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type FeatureSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FeatureSetSpec   `json:"spec,omitempty"`
	Status FeatureSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FeatureSetList contains a list of FeatureSet
type FeatureSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FeatureSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FeatureSet{}, &FeatureSetList{})
}
