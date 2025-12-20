/*
Copyright 2025.

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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SidecarGeneratorSpec defines the desired state of SidecarGenerator.
type SidecarGeneratorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// URL is the URL where to make a GET requst to fetch dependency information.
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// RefreshPeriod is how often the operator will perform a GET request to the specified URL to then update a Sidecar.
	// +kubebuilder:validation:Required
	// +kubebuilder:default="1h"
	RefreshPeriod *metav1.Duration `json:"refreshPeriod,omitempty"`
}

// SidecarGeneratorStatus defines the observed state of SidecarGenerator.
type SidecarGeneratorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	//LastSidecarGeneratorUpdate the last time when the URL was fetched and sidecar updated.
	LastSidecarGeneratorUpdate *metav1.Time `json:"lastSidecarGeneratorUpdate,omitempty"`

	//NextSidecarGeneratorUpdate the next time when the URL will be fetched to update the sidecar.
	NextSidecarGeneratorUpdate *metav1.Time `json:"nextSidecarGeneratorUpdate,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SidecarGenerator is the Schema for the sidecargenerators API.
type SidecarGenerator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SidecarGeneratorSpec   `json:"spec,omitempty"`
	Status SidecarGeneratorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SidecarGeneratorList contains a list of SidecarGenerator.
type SidecarGeneratorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SidecarGenerator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SidecarGenerator{}, &SidecarGeneratorList{})
}
