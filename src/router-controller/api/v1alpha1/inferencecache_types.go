package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InferenceCacheSpec defines the desired state of InferenceCache
type InferenceCacheSpec struct {
	// ModelName is the name of the model to cache
	ModelName string `json:"modelName"`

	// Image is the container image to use
	Image string `json:"image"`

	// TensorParallelSize is the number of GPUs to use for tensor parallelism
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=8
	TensorParallelSize int32 `json:"tensorParallelSize"`

	// CacheType is the type of cache to use
	// +kubebuilder:validation:Enum=kv;attention;hybrid
	CacheType string `json:"cacheType"`

	// CacheSize is the size of the cache in tokens
	// +kubebuilder:validation:Minimum=1
	CacheSize int64 `json:"cacheSize"`

	// CacheTTL is the time-to-live for cache entries
	// +kubebuilder:validation:Pattern=^[0-9]+[smhd]$
	CacheTTL string `json:"cacheTTL"`

	// Resources defines the resource requirements for the container
	Resources corev1.ResourceRequirements `json:"resources"`

	// KVCacheTransferPolicy defines the policy for KV cache transfer
	KVCacheTransferPolicy KVCacheTransferPolicy `json:"kvCacheTransferPolicy"`

	// PDDRef is an optional reference to a PrefillDecodingDisaggregation
	// +kubebuilder:validation:Optional
	PDDRef *LocalObjectReference `json:"pddRef,omitempty"`

	// SDRef is an optional reference to a SpeculativeDecoding
	// +kubebuilder:validation:Optional
	SDRef *LocalObjectReference `json:"sdRef,omitempty"`

	// DisaggregatedPrefillConfig defines the configuration for disaggregated prefill
	DisaggregatedPrefillConfig *DisaggregatedPrefillConfig `json:"disaggregatedPrefillConfig,omitempty"`
}

// KVCacheTransferPolicy defines the policy for KV cache transfer
type KVCacheTransferPolicy struct {
	// ThresholdHitRate is the minimum hit rate required to transfer KV cache
	// +kubebuilder:validation:Pattern=^(0|1|0\.[0-9]+)$
	ThresholdHitRate string `json:"thresholdHitRate"`

	// EvictionThreshold is the maximum memory usage before eviction
	// +kubebuilder:validation:Pattern=^(0|1|0\.[0-9]+)$
	EvictionThreshold string `json:"evictionThreshold"`
}

// InferenceCacheStatus defines the observed state of InferenceCache
type InferenceCacheStatus struct {
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastUpdated is the last time the status was updated
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// CacheStats contains statistics about the cache
	CacheStats CacheStats `json:"cacheStats,omitempty"`
}

// CacheStats contains statistics about the cache
type CacheStats struct {
	// HitRate is the current hit rate of the cache
	// +kubebuilder:validation:Pattern=^(0|1|0\.[0-9]+)$
	HitRate string `json:"hitRate"`

	// TotalEntries is the total number of entries in the cache
	TotalEntries int64 `json:"totalEntries"`

	// MemoryUsage is the current memory usage of the cache
	MemoryUsage int64 `json:"memoryUsage"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Model",type="string",JSONPath=".spec.modelName"
//+kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.cacheType"
//+kubebuilder:printcolumn:name="Size",type="integer",JSONPath=".spec.cacheSize"
//+kubebuilder:printcolumn:name="Hit Rate",type="number",JSONPath=".status.cacheStats.hitRate"
//+kubebuilder:printcolumn:name="Entries",type="integer",JSONPath=".status.cacheStats.totalEntries"
//+kubebuilder:printcolumn:name="Memory",type="string",JSONPath=".status.cacheStats.memoryUsage"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:object:generate=true

// InferenceCache is the Schema for the inferencecaches API
type InferenceCache struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferenceCacheSpec   `json:"spec,omitempty"`
	Status InferenceCacheStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:object:generate=true

// InferenceCacheList contains a list of InferenceCache
type InferenceCacheList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferenceCache `json:"items"`
}
