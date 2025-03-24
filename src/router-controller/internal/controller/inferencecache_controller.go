package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	productionstackv1alpha1 "github.com/vllm-project/production-stack/router-controller/api/v1alpha1"
)

// InferenceCacheReconciler reconciles a InferenceCache object
type InferenceCacheReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=inferencecaches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=inferencecaches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=inferencecaches/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *InferenceCacheReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	cache := &productionstackv1alpha1.InferenceCache{}
	err := r.Get(ctx, req.NamespacedName, cache)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get InferenceCache")
		return ctrl.Result{}, err
	}

	// Initialize status if needed
	if cache.Status.Conditions == nil {
		cache.Status.Conditions = []metav1.Condition{}
	}

	// Create or update the VLLM cache deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cache.Name,
			Namespace: cache.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": cache.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": cache.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "vllm-cache",
							Image: cache.Spec.Image,
							Args: []string{
								"--model", cache.Spec.ModelName,
								"--host", "0.0.0.0",
								"--port", "8000",
								"--tensor-parallel-size", fmt.Sprintf("%d", cache.Spec.TensorParallelSize),
								"--cache-type", cache.Spec.CacheType,
								"--cache-size", fmt.Sprintf("%d", cache.Spec.CacheSize),
								"--cache-ttl", cache.Spec.CacheTTL,
								"--dtype", "bfloat16",
								"--max-model-len", "16384",
								"--gpu-memory-utilization", "0.8",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "LMCACHE_USE_EXPERIMENTAL",
									Value: "True",
								},
								{
									Name:  "VLLM_RPC_TIMEOUT",
									Value: "1000000",
								},
								{
									Name:  "HF_HOME",
									Value: "/data",
								},
								{
									Name:  "VLLM_USE_V1",
									Value: "0",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8000,
								},
							},
							Resources: cache.Spec.Resources,
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8000),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       20,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt(8000),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								FailureThreshold:    3,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-volume",
									MountPath: "/etc/vllm",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cache.Name + "-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Create or update the VLLM cache service
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cache.Name,
			Namespace: cache.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": cache.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8000,
					TargetPort: intstr.FromInt(8000),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	// Create or update the config ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cache.Name + "-config",
			Namespace: cache.Namespace,
		},
		Data: map[string]string{
			"config.json": r.generateConfig(cache),
		},
	}

	// Apply the resources
	if err := r.applyResources(ctx, cache, deployment, svc, configMap); err != nil {
		logger.Error(err, "Failed to apply resources")
		return ctrl.Result{}, err
	}

	// Update status
	cache.Status.LastUpdated = &metav1.Time{Time: time.Now()}
	if err := r.Status().Update(ctx, cache); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// generateConfig generates the configuration for the VLLM cache service
func (r *InferenceCacheReconciler) generateConfig(cache *productionstackv1alpha1.InferenceCache) string {
	config := map[string]interface{}{
		"model":                  cache.Spec.ModelName,
		"host":                   "0.0.0.0",
		"port":                   8000,
		"tensor_parallel_size":   cache.Spec.TensorParallelSize,
		"cache_type":             cache.Spec.CacheType,
		"cache_size":             cache.Spec.CacheSize,
		"cache_ttl":              cache.Spec.CacheTTL,
		"dtype":                  "bfloat16",
		"max_model_len":          16384,
		"gpu_memory_utilization": 0.8,
		"lmcache": map[string]interface{}{
			"enabled":      true,
			"experimental": true,
			"rpc_timeout":  1000000,
		},
	}

	// Add KV cache transfer policy if specified
	if cache.Spec.KVCacheTransferPolicy.ThresholdHitRate != "" {
		config["kv_cache_transfer_policy"] = map[string]interface{}{
			"threshold_hit_rate": cache.Spec.KVCacheTransferPolicy.ThresholdHitRate,
			"eviction_threshold": cache.Spec.KVCacheTransferPolicy.EvictionThreshold,
		}
	}

	// Add disaggregated prefill config if enabled
	if cache.Spec.DisaggregatedPrefillConfig != nil && cache.Spec.DisaggregatedPrefillConfig.Enabled {
		config["disaggregated_prefill"] = map[string]interface{}{
			"enabled":               true,
			"prefill_instance_type": cache.Spec.DisaggregatedPrefillConfig.PrefillInstanceType,
			"connector_type":        cache.Spec.DisaggregatedPrefillConfig.ConnectorType,
			"connector_config":      cache.Spec.DisaggregatedPrefillConfig.ConnectorConfig,
		}

		if cache.Spec.DisaggregatedPrefillConfig.LookupBufferConfig != nil {
			config["lookup_buffer"] = map[string]interface{}{
				"max_size":        cache.Spec.DisaggregatedPrefillConfig.LookupBufferConfig.MaxSize,
				"eviction_policy": cache.Spec.DisaggregatedPrefillConfig.LookupBufferConfig.EvictionPolicy,
			}
		}
	}

	jsonConfig, _ := json.MarshalIndent(config, "", "  ")
	return string(jsonConfig)
}

// applyResources applies the Kubernetes resources
func (r *InferenceCacheReconciler) applyResources(
	ctx context.Context,
	cache *productionstackv1alpha1.InferenceCache,
	deployment *appsv1.Deployment,
	svc *corev1.Service,
	configMap *corev1.ConfigMap,
) error {
	// Apply ConfigMap
	if err := r.applyConfigMap(ctx, cache, configMap); err != nil {
		return err
	}

	// Apply Deployment
	if err := r.applyDeployment(ctx, cache, deployment); err != nil {
		return err
	}

	// Apply Service
	if err := r.applyService(ctx, cache, svc); err != nil {
		return err
	}

	return nil
}

// applyConfigMap applies the ConfigMap
func (r *InferenceCacheReconciler) applyConfigMap(
	ctx context.Context,
	cache *productionstackv1alpha1.InferenceCache,
	configMap *corev1.ConfigMap,
) error {
	var existingConfigMap corev1.ConfigMap
	err := r.Get(ctx, types.NamespacedName{
		Name:      configMap.Name,
		Namespace: configMap.Namespace,
	}, &existingConfigMap)

	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, configMap)
		}
		return err
	}

	existingConfigMap.Data = configMap.Data
	return r.Update(ctx, &existingConfigMap)
}

// applyDeployment applies the Deployment
func (r *InferenceCacheReconciler) applyDeployment(
	ctx context.Context,
	cache *productionstackv1alpha1.InferenceCache,
	deployment *appsv1.Deployment,
) error {
	var existingDeployment appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{
		Name:      deployment.Name,
		Namespace: deployment.Namespace,
	}, &existingDeployment)

	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, deployment)
		}
		return err
	}

	existingDeployment.Spec = deployment.Spec
	return r.Update(ctx, &existingDeployment)
}

// applyService applies the Service
func (r *InferenceCacheReconciler) applyService(
	ctx context.Context,
	cache *productionstackv1alpha1.InferenceCache,
	svc *corev1.Service,
) error {
	var existingService corev1.Service
	err := r.Get(ctx, types.NamespacedName{
		Name:      svc.Name,
		Namespace: svc.Namespace,
	}, &existingService)

	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, svc)
		}
		return err
	}

	existingService.Spec = svc.Spec
	return r.Update(ctx, &existingService)
}

// SetupWithManager sets up the controller with the Manager.
func (r *InferenceCacheReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&productionstackv1alpha1.InferenceCache{}).
		Complete(r)
}
