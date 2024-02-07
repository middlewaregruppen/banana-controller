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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FeatureOverrideSpec defines the desired state of FeatureOverride
type FeatureOverrideSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Match  Match                 `json:"match,omitempty"`
	Values *runtime.RawExtension `json:"values,omitempty" protobuf:"bytes,10,opt,name=values"`
}

type Match struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Project   string `json:"project,omitempty"`
	Repo      string `json:"repo,omitempty"`
	Path      string `json:"path,omitempty"`
	Revision  string `json:"revision,omitempty"`
}

// FeatureOverrideStatus defines the observed state of FeatureOverride
type FeatureOverrideStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FeatureOverride is the Schema for the featureoverrides API
type FeatureOverride struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FeatureOverrideSpec   `json:"spec,omitempty"`
	Status FeatureOverrideStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FeatureOverrideList contains a list of FeatureOverride
type FeatureOverrideList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FeatureOverride `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FeatureOverride{}, &FeatureOverrideList{})
}
