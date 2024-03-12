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
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// FeatureSpec defines the desired state of Feature
type FeatureSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name is the name of this feature
	Name string `json:"name,omitempty"`

	// Namespace is the namespace of the ArgoCD Application provisioned by the Feature controller
	Namespace string `json:"namespace,omitempty"`

	// Project is the ArgoCD project name
	Project string `json:"project,omitempty"`

	// Repo is the repository URL passed to the provisioned ArgoCD Application
	Repo string `json:"repo,omitempty"`

	// Path is the folder path passed to the provisioned ArgoCD Application
	Path string `json:"path,omitempty"`

	// Version is the version passed to the provisioned ArgoCD Application
	Version string `json:"version,omitempty"`

	// Values is the Helm Values passed to the provisioned ArgoCD Application
	Values *runtime.RawExtension `json:"values,omitempty"`

	// SyncPolicy is used to controll Application sync parameters
	SyncPolicy *argov1alpha1.SyncPolicy `json:"syncPolicy,omitempty"`

	// DisableAutoSync disables `syncPolicy` by removing it from the Argo Application
	DisableAutoSync bool `json:"disableAutoSync,omitempty"`

	// Labels is a set of key value pairs that will exist on the provisioned ArgoCD Application
	Labels map[string]string `json:"labels,omitempty"`
}

// FeatureStatus defines the observed state of Feature
type FeatureStatus struct {
	Conditions    []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	SyncStatus    string             `json:"syncStatus,omitempty"`
	HealthStatus  string             `json:"healthStatus,omitempty"`
	URLs          []string           `json:"url,omitempty"`
	Images        []string           `json:"images,omitempty"`
	NeedsUpdating bool               `json:"needsUpdating,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.syncStatus"
// +kubebuilder:printcolumn:name="Health",type="string",JSONPath=".status.healthStatus"
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.url[0]"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:rbac:groups=banana.mdlwr.se,resources=features,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banana.mdlwr.se,resources=features/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=banana.mdlwr.se,resources=features/finalizers,verbs=update
// +kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=argoproj.io,resources=applications/status,verbs=get
// +kubebuilder:rbac:groups=banana.mdlwr.se,resources=featureoverrides,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banana.mdlwr.se,resources=featureoverrides/status,verbs=get

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
