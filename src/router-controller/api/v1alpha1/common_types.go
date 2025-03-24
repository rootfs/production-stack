package v1alpha1

// DisaggregatedPrefillConfig defines the configuration for disaggregated prefill
type DisaggregatedPrefillConfig struct {
	// Enabled enables disaggregated prefill
	Enabled bool `json:"enabled"`

	// PrefillInstanceType is the type of instance to use for prefill
	// +kubebuilder:validation:Enum=prefill;decode
	PrefillInstanceType string `json:"prefillInstanceType"`

	// ConnectorType is the type of connector to use for KV cache transfer
	// +kubebuilder:validation:Enum=grpc;redis;custom
	ConnectorType string `json:"connectorType"`

	// ConnectorConfig is the configuration for the connector
	ConnectorConfig map[string]string `json:"connectorConfig,omitempty"`

	// LookupBufferConfig is the configuration for the lookup buffer
	LookupBufferConfig *LookupBufferConfig `json:"lookupBufferConfig,omitempty"`
}

// LookupBufferConfig defines the configuration for the lookup buffer
type LookupBufferConfig struct {
	// MaxSize is the maximum size of the lookup buffer
	MaxSize int64 `json:"maxSize"`

	// EvictionPolicy is the policy for evicting entries from the buffer
	// +kubebuilder:validation:Enum=lru;fifo
	EvictionPolicy string `json:"evictionPolicy"`
}

// SpeculativeDecodingConfig defines the configuration for speculative decoding
type SpeculativeDecodingConfig struct {
	// Enabled enables speculative decoding
	Enabled bool `json:"enabled"`

	// SpeculationType is the type of speculative decoding to use
	// +kubebuilder:validation:Enum=draft;ngram;mlp;eagle
	SpeculationType string `json:"speculationType"`

	// DraftModel is the name of the draft model to use for speculation
	// Required for draft, MLP, and EAGLE speculation types
	DraftModel string `json:"draftModel,omitempty"`

	// NumSpeculativeTokens is the number of tokens to speculate
	// +kubebuilder:validation:Minimum=1
	NumSpeculativeTokens int32 `json:"numSpeculativeTokens"`

	// DraftTensorParallelSize is the tensor parallel size for the draft model
	// Required for MLP and EAGLE speculation types
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=8
	DraftTensorParallelSize int32 `json:"draftTensorParallelSize,omitempty"`

	// PromptLookupMax is the maximum n-gram size for prompt lookup
	// Required for n-gram speculation type
	// +kubebuilder:validation:Minimum=1
	PromptLookupMax int32 `json:"promptLookupMax,omitempty"`

	// DraftModelConfig is additional configuration for the draft model
	DraftModelConfig map[string]string `json:"draftModelConfig,omitempty"`
}

// BackendRef defines the configuration for the VLLM backend service
type BackendRef struct {
	// Group is the group of the referent. For Envoy Gateway Backend, this is "gateway.envoyproxy.io"
	// +kubebuilder:validation:Enum=gateway.envoyproxy.io
	Group string `json:"group"`

	// Kind is the kind of the referent. For Envoy Gateway Backend, this is "Backend"
	// +kubebuilder:validation:Enum=Backend
	Kind string `json:"kind"`

	// Name is the name of the referent
	Name string `json:"name"`

	// Port is the port of the referent
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Weight is the weight of the backend
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Weight int32 `json:"weight,omitempty"`
}
