package engine

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SGLangFactory creates SGLang engine controllers
type SGLangFactory struct{}

// CreateController creates a new SGLang engine controller
func (f *SGLangFactory) CreateController(client client.Client, scheme *runtime.Scheme) (EngineController, error) {
	return &SGLangController{
		client: client,
		scheme: scheme,
	}, nil
}

// SGLangController implements the EngineController interface for SGLang
type SGLangController struct {
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reconciles SGLang-specific resources
func (c *SGLangController) Reconcile(ctx context.Context, config EngineConfig) error {
	// TODO: Implement SGLang-specific reconciliation logic
	return nil
}

// Cleanup cleans up SGLang-specific resources
func (c *SGLangController) Cleanup(ctx context.Context, config EngineConfig) error {
	// TODO: Implement SGLang-specific cleanup logic
	return nil
}

// GetPodSpec returns the pod spec for SGLang
func (c *SGLangController) GetPodSpec(config EngineConfig) (*corev1.PodSpec, error) {
	args := []string{
		"--model", config.ModelName,
		"--host", "0.0.0.0",
		"--port", "8000",
	}

	// Add engine-specific configuration
	for key, value := range config.EngineSpecificConfig {
		args = append(args, fmt.Sprintf("--%s", key), value)
	}

	return &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "sglang",
				Image: "sglang:latest", // TODO: Make configurable
				Args:  args,
				Resources: corev1.ResourceRequirements{
					Requests: config.Resources.Requests,
					Limits:   config.Resources.Limits,
				},
			},
		},
	}, nil
}
