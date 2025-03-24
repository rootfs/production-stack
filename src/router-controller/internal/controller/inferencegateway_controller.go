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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	productionstackv1alpha1 "github.com/vllm-project/production-stack/router-controller/api/v1alpha1"
)

// InferenceGatewayReconciler reconciles a InferenceGateway object
type InferenceGatewayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=inferencegateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=inferencegateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=inferencegateways/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *InferenceGatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	gateway := &productionstackv1alpha1.InferenceGateway{}
	err := r.Get(ctx, req.NamespacedName, gateway)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get InferenceGateway")
		return ctrl.Result{}, err
	}

	// Initialize status if needed
	if gateway.Status.Conditions == nil {
		gateway.Status.Conditions = []metav1.Condition{}
	}

	// Create or update the router deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.Name,
			Namespace: gateway.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": gateway.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": gateway.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "vllm-router",
							Image: "lmcache/test:router",
							Args: []string{
								"--dynamic-config-json",
								"/etc/vllm-router/dynamic_config.json",
								"--service-discovery",
								gateway.Spec.ServiceDiscovery,
								"--routing-logic",
								gateway.Spec.RouteStrategy,
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8001,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8001),
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
										Port: intstr.FromInt(8001),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								FailureThreshold:    3,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-volume",
									MountPath: "/etc/vllm-router",
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
										Name: gateway.Name + "-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Create or update the router service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.Name,
			Namespace: gateway.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": gateway.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8001,
					TargetPort: intstr.FromInt(8001),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	// Create or update the dynamic config ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config", gateway.Name),
			Namespace: gateway.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: gateway.APIVersion,
					Kind:       gateway.Kind,
					Name:       gateway.Name,
					UID:        gateway.UID,
					Controller: pointer.Bool(true),
				},
			},
		},
		Data: map[string]string{},
	}

	config, err := r.generateConfig(gateway)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to generate config: %w", err)
	}

	configMap.Data["dynamic_config.json"] = config

	// Apply the resources
	if err := r.applyResources(ctx, gateway, deployment, service, configMap); err != nil {
		logger.Error(err, "Failed to apply resources")
		return ctrl.Result{}, err
	}

	// Update status
	gateway.Status.LastUpdated = &metav1.Time{Time: time.Now()}
	if err := r.Status().Update(ctx, gateway); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// generateConfig generates the dynamic configuration for the router
func (r *InferenceGatewayReconciler) generateConfig(gateway *productionstackv1alpha1.InferenceGateway) (string, error) {
	config := make(map[string]interface{})

	// Add service discovery specific config
	switch gateway.Spec.ServiceDiscovery {
	case "static":
		var backends, models []string
		var cacheConfigs []map[string]interface{}

		for _, route := range gateway.Spec.ModelRoutes {
			// Get the referenced InferenceService - must be in same namespace as Gateway
			service := &productionstackv1alpha1.InferenceService{}
			if err := r.Get(context.Background(), types.NamespacedName{
				Name:      route.ServiceRef.Name,
				Namespace: gateway.Namespace,
			}, service); err != nil {
				return "", fmt.Errorf("failed to get InferenceService %s: %w", route.ServiceRef.Name, err)
			}

			// Verify the reference is to an InferenceService
			if route.ServiceRef.APIGroup != "production-stack.vllm.ai" || route.ServiceRef.Kind != "InferenceService" {
				return "", fmt.Errorf("invalid service reference: must reference an InferenceService, got %s/%s",
					route.ServiceRef.APIGroup, route.ServiceRef.Kind)
			}

			// Get the backend service from the InferenceService
			backends = append(backends, fmt.Sprintf("http://%s.%s:8000", service.Spec.BackendRef.Name, gateway.Namespace))
			models = append(models, route.ModelName)

			// If service has cache config, add it to cache configs
			if service.Spec.InferenceCacheRef != nil {
				cacheConfigs = append(cacheConfigs, map[string]interface{}{
					"model": route.ModelName,
					"cache": map[string]string{
						"name":      service.Spec.InferenceCacheRef.Name,
						"namespace": gateway.Namespace,
					},
				})
			}
		}
		config["static_backends"] = strings.Join(backends, ",")
		config["static_models"] = strings.Join(models, ",")
		if len(cacheConfigs) > 0 {
			config["cache_configs"] = cacheConfigs
		}
	case "k8s":
		config["k8s_namespace"] = gateway.Namespace
		config["k8s_port"] = 8000
		config["k8s_label_selector"] = "app=vllm-service"
	}

	// Add routing logic specific config
	if gateway.Spec.RouteStrategy == "session-affinity" {
		config["session_key"] = "session_id"
	}

	jsonConfig, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}
	return string(jsonConfig), nil
}

// applyResources applies the Kubernetes resources
func (r *InferenceGatewayReconciler) applyResources(
	ctx context.Context,
	gateway *productionstackv1alpha1.InferenceGateway,
	deployment *appsv1.Deployment,
	service *corev1.Service,
	configMap *corev1.ConfigMap,
) error {
	// Apply ConfigMap
	if err := r.applyConfigMap(ctx, gateway, configMap); err != nil {
		return err
	}

	// Apply Deployment
	if err := r.applyDeployment(ctx, gateway, deployment); err != nil {
		return err
	}

	// Apply Service
	if err := r.applyService(ctx, gateway, service); err != nil {
		return err
	}

	return nil
}

// applyConfigMap applies the ConfigMap
func (r *InferenceGatewayReconciler) applyConfigMap(
	ctx context.Context,
	gateway *productionstackv1alpha1.InferenceGateway,
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
func (r *InferenceGatewayReconciler) applyDeployment(
	ctx context.Context,
	gateway *productionstackv1alpha1.InferenceGateway,
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
func (r *InferenceGatewayReconciler) applyService(
	ctx context.Context,
	gateway *productionstackv1alpha1.InferenceGateway,
	service *corev1.Service,
) error {
	var existingService corev1.Service
	err := r.Get(ctx, types.NamespacedName{
		Name:      service.Name,
		Namespace: service.Namespace,
	}, &existingService)

	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, service)
		}
		return err
	}

	existingService.Spec = service.Spec
	return r.Update(ctx, &existingService)
}

// findInferenceGatewaysForInferenceService implements handler.MapFunc
func (r *InferenceGatewayReconciler) findInferenceGatewaysForInferenceService(ctx context.Context, obj client.Object) []reconcile.Request {
	service, ok := obj.(*productionstackv1alpha1.InferenceService)
	if !ok {
		return nil
	}

	gateways := &productionstackv1alpha1.InferenceGatewayList{}
	if err := r.List(ctx, gateways); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, gateway := range gateways.Items {
		for _, route := range gateway.Spec.ModelRoutes {
			// Since LocalObjectReference is namespace-scoped, we only need to check the name
			// and that the gateway is in the same namespace as the service
			if route.ServiceRef.Name == service.Name && gateway.Namespace == service.Namespace {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      gateway.Name,
						Namespace: gateway.Namespace,
					},
				})
				break
			}
		}
	}
	return requests
}

// findInferenceGatewaysForInferenceCache implements handler.MapFunc
func (r *InferenceGatewayReconciler) findInferenceGatewaysForInferenceCache(ctx context.Context, obj client.Object) []reconcile.Request {
	cache, ok := obj.(*productionstackv1alpha1.InferenceCache)
	if !ok {
		return nil
	}

	gateways := &productionstackv1alpha1.InferenceGatewayList{}
	if err := r.List(ctx, gateways); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, gateway := range gateways.Items {
		for _, route := range gateway.Spec.ModelRoutes {
			// Get the InferenceService from the same namespace as the gateway
			service := &productionstackv1alpha1.InferenceService{}
			if err := r.Get(ctx, types.NamespacedName{
				Name:      route.ServiceRef.Name,
				Namespace: gateway.Namespace,
			}, service); err != nil {
				continue
			}

			if service.Spec.InferenceCacheRef != nil &&
				service.Spec.InferenceCacheRef.Name == cache.Name &&
				service.Namespace == cache.Namespace {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      gateway.Name,
						Namespace: gateway.Namespace,
					},
				})
				break
			}
		}
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *InferenceGatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&productionstackv1alpha1.InferenceGateway{}).
		Owns(&corev1.ConfigMap{}).
		// Watch InferenceServices and trigger reconciliation of referencing Gateways
		Watches(
			&productionstackv1alpha1.InferenceService{},
			handler.EnqueueRequestsFromMapFunc(r.findInferenceGatewaysForInferenceService),
		).
		// Watch InferenceCaches and trigger reconciliation of referencing Gateways
		Watches(
			&productionstackv1alpha1.InferenceCache{},
			handler.EnqueueRequestsFromMapFunc(r.findInferenceGatewaysForInferenceCache),
		).
		Complete(r)
}

// mapServiceToGateway maps a Service to a list of InferenceGateways that reference it
func (r *InferenceGatewayReconciler) mapServiceToGateway(service client.Object) []reconcile.Request {
	var requests []reconcile.Request

	// Get all InferenceGateways in the cluster
	gateways := &productionstackv1alpha1.InferenceGatewayList{}
	if err := r.List(context.Background(), gateways); err != nil {
		return nil
	}

	// Find all InferenceGateways that reference this service
	for _, gateway := range gateways.Items {
		for _, route := range gateway.Spec.ModelRoutes {
			// Since LocalObjectReference is namespace-scoped, we only need to check the name
			if route.ServiceRef.Name == service.GetName() {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      gateway.Name,
						Namespace: gateway.Namespace,
					},
				})
				break
			}
		}
	}

	return requests
}
