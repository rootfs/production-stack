package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SpeculativeDecodingSpec defines the desired state of SpeculativeDecoding
type SpeculativeDecodingSpec struct {
	// SpeculationType defines the type of speculative decoding to use
	// +kubebuilder:validation:Enum=draft;ngram;mlp;eagle
	SpeculationType string `json:"speculationType"`

	// DraftModel is the name of the draft model to use (for draft, mlp, and eagle types)
	// +kubebuilder:validation:Optional
	DraftModel string `json:"draftModel,omitempty"`

	// NumSpeculativeTokens is the number of tokens to speculate
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=32
	NumSpeculativeTokens int `json:"numSpeculativeTokens"`

	// DraftTensorParallelSize is the tensor parallel size for the draft model
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	DraftTensorParallelSize *int `json:"draftTensorParallelSize,omitempty"`

	// PromptLookupMax is the maximum number of tokens to look up in the prompt (for ngram type)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=8
	PromptLookupMax *int `json:"promptLookupMax,omitempty"`

	// DraftModelConfig contains configuration for the draft model
	// +kubebuilder:validation:Optional
	DraftModelConfig *DraftModelConfig `json:"draftModelConfig,omitempty"`
}

// DraftModelConfig defines configuration for the draft model
type DraftModelConfig struct {
	// Temperature controls randomness in generation
	// +kubebuilder:validation:Pattern=^(0|1|0\.[0-9]+)$
	Temperature string `json:"temperature"`

	// MaxTokens is the maximum number of tokens to generate
	// +kubebuilder:validation:Pattern=^\d+$
	MaxTokens string `json:"max_tokens"`
}

// SpeculativeDecodingStatus defines the observed state of SpeculativeDecoding
type SpeculativeDecodingStatus struct {
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// DraftPodName is the name of the draft model pod
	DraftPodName string `json:"draftPodName,omitempty"`

	// TargetPodName is the name of the target model pod
	TargetPodName string `json:"targetPodName,omitempty"`

	// LastUpdated is the last time the status was updated
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Stats contains statistics about speculative decoding performance
	Stats SpeculativeDecodingStats `json:"stats,omitempty"`
}

// SpeculativeDecodingStats contains statistics about speculative decoding performance
type SpeculativeDecodingStats struct {
	// AcceptanceRate is the rate at which draft tokens are accepted
	// +kubebuilder:validation:Pattern=^(0|1|0\.[0-9]+)$
	AcceptanceRate string `json:"acceptanceRate"`

	// AverageSpeedup is the average speedup achieved
	// +kubebuilder:validation:Pattern=^[0-9]+(\.[0-9]+)?$
	AverageSpeedup string `json:"averageSpeedup"`

	// TotalTokensGenerated is the total number of tokens generated
	TotalTokensGenerated int64 `json:"totalTokensGenerated"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.speculationType"
//+kubebuilder:printcolumn:name="Draft Model",type="string",JSONPath=".spec.draftModel"
//+kubebuilder:printcolumn:name="Tokens",type="integer",JSONPath=".spec.numSpeculativeTokens"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:object:generate=true

// SpeculativeDecoding is the Schema for the speculativedecodings API
type SpeculativeDecoding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SpeculativeDecodingSpec   `json:"spec,omitempty"`
	Status SpeculativeDecodingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:object:generate=true

// SpeculativeDecodingList contains a list of SpeculativeDecoding
type SpeculativeDecodingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpeculativeDecoding `json:"items"`
}
