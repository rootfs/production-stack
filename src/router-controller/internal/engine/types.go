package engine

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EngineType represents the type of inference engine
type EngineType string

const (
	// EngineTypeVLLM represents the vLLM engine
	EngineTypeVLLM EngineType = "vllm"
	// EngineTypeSGLang represents the SGLang engine
	EngineTypeSGLang EngineType = "sglang"
)

// EngineConfig represents the configuration for an inference engine
type EngineConfig struct {
	// ModelName is the name of the model to serve
	ModelName string `json:"modelName"`

	// Resources defines the resource requirements
	Resources corev1.ResourceRequirements `json:"resources"`

	// EngineSpecificConfig contains engine-specific configuration
	EngineSpecificConfig map[string]string `json:"engineSpecificConfig,omitempty"`
}

// EngineController defines the interface for inference engine controllers
type EngineController interface {
	// Reconcile reconciles the engine-specific resources
	Reconcile(ctx context.Context, config EngineConfig) error

	// Cleanup cleans up engine-specific resources
	Cleanup(ctx context.Context, config EngineConfig) error

	// GetPodSpec returns the pod spec for the engine
	GetPodSpec(config EngineConfig) (*corev1.PodSpec, error)
}

// EngineFactory creates engine controllers
type EngineFactory interface {
	// CreateController creates a new engine controller
	CreateController(client client.Client, scheme *runtime.Scheme) (EngineController, error)
}

// NewEngineFactory creates a new engine factory
func NewEngineFactory(engineType EngineType) (EngineFactory, error) {
	switch engineType {
	case EngineTypeVLLM:
		return &VLLMFactory{}, nil
	case EngineTypeSGLang:
		return &SGLangFactory{}, nil
	default:
		return nil, fmt.Errorf("unsupported engine type: %s", engineType)
	}
}
