package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PrefillDecodingDisaggregationSpec defines the desired state of PrefillDecodingDisaggregation
type PrefillDecodingDisaggregationSpec struct {
	// ModelName is the name of the model to serve
	ModelName string `json:"modelName"`

	// TopologyHint provides hints about the desired topology
	TopologyHint TopologyHint `json:"topologyHint"`

	// PrefillResources defines the resource requirements for the prefill stage
	PrefillResources corev1.ResourceRequirements `json:"prefillResources"`

	// DecodeResources defines the resource requirements for the decode stage
	DecodeResources corev1.ResourceRequirements `json:"decodeResources"`
}

// TopologyHint provides hints about the desired topology
type TopologyHint struct {
	// NodeSelector is a selector for nodes
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Affinity is the pod affinity
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations are the pod tolerations
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// PrefillDecodingDisaggregationStatus defines the observed state of PrefillDecodingDisaggregation
type PrefillDecodingDisaggregationStatus struct {
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// PrefillPodName is the name of the prefill pod
	PrefillPodName string `json:"prefillPodName,omitempty"`

	// DecodePodName is the name of the decode pod
	DecodePodName string `json:"decodePodName,omitempty"`

	// LastUpdated is the last time the status was updated
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Model",type="string",JSONPath=".spec.modelName"
//+kubebuilder:printcolumn:name="Prefill Pod",type="string",JSONPath=".status.prefillPodName"
//+kubebuilder:printcolumn:name="Decode Pod",type="string",JSONPath=".status.decodePodName"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:object:generate=true

// PrefillDecodingDisaggregation is the Schema for the prefilldecodingdisaggregations API
type PrefillDecodingDisaggregation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PrefillDecodingDisaggregationSpec   `json:"spec,omitempty"`
	Status PrefillDecodingDisaggregationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:object:generate=true

// PrefillDecodingDisaggregationList contains a list of PrefillDecodingDisaggregation
type PrefillDecodingDisaggregationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrefillDecodingDisaggregation `json:"items"`
}
