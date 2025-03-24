package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	productionstackv1alpha1 "github.com/vllm-project/production-stack/router-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

// InferenceServiceReconciler reconciles a InferenceService object
type InferenceServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=inferenceservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=inferenceservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=inferenceservices/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.envoyproxy.io,resources=backends,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *InferenceServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	service := &productionstackv1alpha1.InferenceService{}
	err := r.Get(ctx, req.NamespacedName, service)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get InferenceService")
		return ctrl.Result{}, err
	}

	// Initialize status if needed
	if service.Status.Conditions == nil {
		service.Status.Conditions = []metav1.Condition{}
	}

	// Get the referenced Envoy Gateway Backend
	backend := &corev1.Service{}
	backendRef := service.Spec.BackendRef
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: service.Namespace,
		Name:      backendRef.Name,
	}, backend); err != nil {
		logger.Error(err, "Failed to get backend service")
		return ctrl.Result{}, err
	}

	// Create or update the VLLM service based on the backend configuration
	if err := r.reconcileVLLMService(ctx, service, backend); err != nil {
		logger.Error(err, "Failed to reconcile VLLM service")
		return ctrl.Result{}, err
	}

	// Update the backend service with the VLLM service details
	if err := r.updateBackend(ctx, service, backend); err != nil {
		logger.Error(err, "Failed to update backend service")
		return ctrl.Result{}, err
	}

	// Update status
	service.Status.LastUpdated = &metav1.Time{Time: time.Now()}
	if err := r.Status().Update(ctx, service); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileVLLMService creates or updates the VLLM service based on the backend configuration
func (r *InferenceServiceReconciler) reconcileVLLMService(ctx context.Context, service *productionstackv1alpha1.InferenceService, backend *corev1.Service) error {
	// Create service configuration based on backend endpoints
	serviceConfig := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: service.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: service.APIVersion,
					Kind:       service.Kind,
					Name:       service.Name,
					UID:        service.UID,
					Controller: pointer.Bool(true),
				},
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       service.Spec.BackendRef.Port,
					TargetPort: intstr.FromInt(int(service.Spec.BackendRef.Port)),
				},
			},
			Selector: map[string]string{
				"app": service.Name,
			},
		},
	}

	// Create or update the service
	if err := r.Create(ctx, serviceConfig); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
		if err := r.Update(ctx, serviceConfig); err != nil {
			return err
		}
	}

	// Create deployment configuration
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: service.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: service.APIVersion,
					Kind:       service.Kind,
					Name:       service.Name,
					UID:        service.UID,
					Controller: pointer.Bool(true),
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": service.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": service.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "vllm",
							Image: "vllm/vllm-openai:latest",
							Args:  r.generateVLLMArgs(service),
							Env:   r.generateVLLMEnv(service),
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("4"),
									corev1.ResourceMemory: resource.MustParse("16Gi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("8Gi"),
								},
							},
						},
					},
				},
			},
		},
	}

	// Create or update the deployment
	if err := r.Create(ctx, deployment); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
		if err := r.Update(ctx, deployment); err != nil {
			return err
		}
	}

	return nil
}

// generateVLLMArgs generates the VLLM command line arguments
func (r *InferenceServiceReconciler) generateVLLMArgs(service *productionstackv1alpha1.InferenceService) []string {
	args := []string{
		"--model", service.Spec.ModelName,
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", service.Spec.BackendRef.Port),
	}

	// Add prefill decoding configuration if referenced
	if service.Spec.PrefillDecodingRef != nil {
		pdd := &productionstackv1alpha1.PrefillDecodingDisaggregation{}
		if err := r.Get(context.Background(), types.NamespacedName{
			Name:      service.Spec.PrefillDecodingRef.Name,
			Namespace: service.Namespace,
		}, pdd); err == nil {
			// Get the GPU resource request for prefill
			gpuResource := "g4dn.xlarge" // default instance type
			for resourceName, quantity := range pdd.Spec.PrefillResources.Requests {
				if strings.Contains(string(resourceName), "nvidia.com/gpu") {
					gpuResource = fmt.Sprintf("g%d.xlarge", quantity.Value())
					break
				}
			}

			args = append(args,
				"--disaggregated-prefill",
				"--prefill-instance-type", gpuResource,
			)
		}
	}

	// Add speculative decoding configuration if referenced
	if service.Spec.SpeculativeDecodingRef != nil {
		sd := &productionstackv1alpha1.SpeculativeDecoding{}
		if err := r.Get(context.Background(), types.NamespacedName{
			Name:      service.Spec.SpeculativeDecodingRef.Name,
			Namespace: service.Namespace,
		}, sd); err == nil {
			args = append(args,
				"--speculative-config", r.generateSpeculativeConfig(sd),
			)
		}
	}

	return args
}

// generateVLLMEnv generates the environment variables for VLLM
func (r *InferenceServiceReconciler) generateVLLMEnv(service *productionstackv1alpha1.InferenceService) []corev1.EnvVar {
	env := []corev1.EnvVar{}

	// Add topology hints if prefill decoding is referenced
	if service.Spec.PrefillDecodingRef != nil {
		pdd := &productionstackv1alpha1.PrefillDecodingDisaggregation{}
		if err := r.Get(context.Background(), types.NamespacedName{
			Name:      service.Spec.PrefillDecodingRef.Name,
			Namespace: service.Namespace,
		}, pdd); err == nil && len(pdd.Spec.TopologyHint.NodeSelector) > 0 {
			for k, v := range pdd.Spec.TopologyHint.NodeSelector {
				env = append(env, corev1.EnvVar{
					Name:  fmt.Sprintf("NODE_%s", strings.ToUpper(k)),
					Value: v,
				})
			}
		}
	}

	return env
}

// generateSpeculativeConfig generates the speculative decoding configuration JSON
func (r *InferenceServiceReconciler) generateSpeculativeConfig(sd *productionstackv1alpha1.SpeculativeDecoding) string {
	specConfig := map[string]interface{}{
		"type":                   sd.Spec.SpeculationType,
		"model":                  sd.Spec.DraftModel,
		"num_speculative_tokens": sd.Spec.NumSpeculativeTokens,
	}

	if sd.Spec.DraftTensorParallelSize != nil {
		specConfig["draft_tensor_parallel_size"] = *sd.Spec.DraftTensorParallelSize
	}

	if sd.Spec.PromptLookupMax != nil {
		specConfig["prompt_lookup_max"] = *sd.Spec.PromptLookupMax
	}

	if sd.Spec.DraftModelConfig != nil {
		specConfig["temperature"] = sd.Spec.DraftModelConfig.Temperature
		specConfig["max_tokens"] = sd.Spec.DraftModelConfig.MaxTokens
	}

	jsonConfig, _ := json.Marshal(specConfig)
	return string(jsonConfig)
}

// updateBackend updates the backend service with the VLLM service details
func (r *InferenceServiceReconciler) updateBackend(ctx context.Context, service *productionstackv1alpha1.InferenceService, backend *corev1.Service) error {
	// Update the backend service's endpoints
	backend.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       service.Spec.BackendRef.Port,
			TargetPort: intstr.FromInt(int(service.Spec.BackendRef.Port)),
		},
	}

	// Update the backend service
	if err := r.Update(ctx, backend); err != nil {
		return fmt.Errorf("failed to update backend service: %w", err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InferenceServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&productionstackv1alpha1.InferenceService{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
