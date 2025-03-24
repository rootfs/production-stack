package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InferenceGatewaySpec defines the desired state of InferenceGateway
type InferenceGatewaySpec struct {
	// ServiceDiscovery specifies how to discover inference services
	// +kubebuilder:validation:Enum=static;k8s
	ServiceDiscovery string `json:"serviceDiscovery"`

	// RouteStrategy specifies how to route requests to services
	// +kubebuilder:validation:Enum=round-robin;session-affinity;prefix-hash
	RouteStrategy string `json:"routeStrategy"`

	// ModelRoutes specifies the mapping of models to services
	ModelRoutes []ModelRoute `json:"modelRoutes"`

	// TopologyHint provides scheduling hints for the gateway
	TopologyHint *TopologyHint `json:"topologyHint,omitempty"`
}

// ModelRoute defines a mapping between a model and a service
type ModelRoute struct {
	// ModelName is the name of the model
	ModelName string `json:"modelName"`

	// ServiceRef is a reference to an InferenceService
	// +kubebuilder:validation:Required
	ServiceRef LocalObjectReference `json:"serviceRef"`
}

// LocalObjectReference is a reference to another object in the same namespace
type LocalObjectReference struct {
	// Name is the name of the referenced object
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// APIGroup is the group for the resource being referenced
	// +kubebuilder:validation:Enum=production-stack.vllm.ai
	APIGroup string `json:"apiGroup"`

	// Kind is the kind of resource being referenced
	// +kubebuilder:validation:Enum=InferenceService
	Kind string `json:"kind"`
}

// InferenceGatewayStatus defines the observed state of InferenceGateway
type InferenceGatewayStatus struct {
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastUpdated is the last time the status was updated
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Scheduling",type="string",JSONPath=".spec.schedulingPolicy"
//+kubebuilder:printcolumn:name="Strategy",type="string",JSONPath=".spec.routeStrategy"
//+kubebuilder:printcolumn:name="Affinity",type="boolean",JSONPath=".spec.sessionAffinity"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:object:generate=true

// InferenceGateway is the Schema for the inferencegateways API
type InferenceGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferenceGatewaySpec   `json:"spec,omitempty"`
	Status InferenceGatewayStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:object:generate=true

// InferenceGatewayList contains a list of InferenceGateway
type InferenceGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferenceGateway `json:"items"`
}
