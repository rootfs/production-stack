package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InferenceServiceSpec defines the desired state of InferenceService
type InferenceServiceSpec struct {
	// ModelName is the name of the model to serve
	ModelName string `json:"modelName"`

	// BackendRef is a reference to the Envoy Gateway Backend
	BackendRef BackendRef `json:"backendRef"`

	// InferenceCacheRef is an optional reference to an InferenceCache
	// +kubebuilder:validation:Optional
	InferenceCacheRef *LocalObjectReference `json:"inferenceCacheRef,omitempty"`

	// NLPFilters defines the NLP filters to apply
	NLPFilters []NLPAction `json:"nlpFilters,omitempty"`

	// PrefillDecodingRef is an optional reference to a PrefillDecodingDisaggregation resource
	// +kubebuilder:validation:Optional
	PrefillDecodingRef *LocalObjectReference `json:"prefillDecodingRef,omitempty"`

	// SpeculativeDecodingRef is an optional reference to a SpeculativeDecoding resource
	// +kubebuilder:validation:Optional
	SpeculativeDecodingRef *LocalObjectReference `json:"speculativeDecodingRef,omitempty"`
}

// NLPAction defines an NLP action to perform
type NLPAction struct {
	// Type is the type of NLP action
	// +kubebuilder:validation:Enum=tokenLimit;contentFilter;sentimentAnalysis
	Type string `json:"type"`

	// Parameters are the parameters for the action
	Parameters map[string]string `json:"parameters,omitempty"`
}

// InferenceServiceStatus defines the observed state of InferenceService
type InferenceServiceStatus struct {
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastUpdated is the last time the status was updated
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Model",type="string",JSONPath=".spec.modelName"
//+kubebuilder:printcolumn:name="Backend",type="string",JSONPath=".spec.backendRef.name"
//+kubebuilder:printcolumn:name="Cache",type="string",JSONPath=".spec.inferenceCacheRef.name"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:object:generate=true

// InferenceService is the Schema for the inferenceservices API
type InferenceService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferenceServiceSpec   `json:"spec,omitempty"`
	Status InferenceServiceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:object:generate=true

// InferenceServiceList contains a list of InferenceService
type InferenceServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferenceService `json:"items"`
}
