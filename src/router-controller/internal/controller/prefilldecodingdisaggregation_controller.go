package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	productionstackv1alpha1 "github.com/vllm-project/production-stack/router-controller/api/v1alpha1"
)

// PrefillDecodingDisaggregationReconciler reconciles a PrefillDecodingDisaggregation object
type PrefillDecodingDisaggregationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=prefilldecodingdisaggregations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=prefilldecodingdisaggregations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=prefilldecodingdisaggregations/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PrefillDecodingDisaggregationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	pdd := &productionstackv1alpha1.PrefillDecodingDisaggregation{}

	if err := r.Get(ctx, req.NamespacedName, pdd); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("PDD not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get PDD")
		return ctrl.Result{}, err
	}

	// Initialize status if needed
	if pdd.Status.Conditions == nil {
		pdd.Status.Conditions = []metav1.Condition{}
	}

	// Create or update prefill pod
	if err := r.reconcilePrefillPod(ctx, pdd); err != nil {
		logger.Error(err, "Failed to reconcile prefill pod")
		pdd.Status.Conditions = append(pdd.Status.Conditions, metav1.Condition{
			Type:               "PrefillReady",
			Status:             metav1.ConditionFalse,
			Reason:             "PodError",
			Message:            err.Error(),
			LastTransitionTime: metav1.Now(),
		})
		if err := r.Status().Update(ctx, pdd); err != nil {
			logger.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	// Create or update decode pod
	if err := r.reconcileDecodePod(ctx, pdd); err != nil {
		logger.Error(err, "Failed to reconcile decode pod")
		pdd.Status.Conditions = append(pdd.Status.Conditions, metav1.Condition{
			Type:               "DecodeReady",
			Status:             metav1.ConditionFalse,
			Reason:             "PodError",
			Message:            err.Error(),
			LastTransitionTime: metav1.Now(),
		})
		if err := r.Status().Update(ctx, pdd); err != nil {
			logger.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	// Create or update service
	if err := r.reconcileService(ctx, pdd); err != nil {
		logger.Error(err, "Failed to reconcile service")
		pdd.Status.Conditions = append(pdd.Status.Conditions, metav1.Condition{
			Type:               "ServiceReady",
			Status:             metav1.ConditionFalse,
			Reason:             "ServiceError",
			Message:            err.Error(),
			LastTransitionTime: metav1.Now(),
		})
		if err := r.Status().Update(ctx, pdd); err != nil {
			logger.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	// Update status
	pdd.Status.Conditions = append(pdd.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "PDD configuration is up to date",
		LastTransitionTime: metav1.Now(),
	})
	pdd.Status.LastUpdated = &metav1.Time{Time: time.Now()}

	if err := r.Status().Update(ctx, pdd); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully reconciled PDD")
	return ctrl.Result{}, nil
}

// reconcilePrefillPod creates or updates the prefill pod
func (r *PrefillDecodingDisaggregationReconciler) reconcilePrefillPod(ctx context.Context, pdd *productionstackv1alpha1.PrefillDecodingDisaggregation) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-prefill", pdd.Name),
			Namespace: pdd.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "prefill",
					Image: "vllm-prefill:latest", // TODO: Make configurable
					Resources: corev1.ResourceRequirements{
						Requests: pdd.Spec.PrefillResources.Requests,
						Limits:   pdd.Spec.PrefillResources.Limits,
					},
				},
			},
			NodeSelector: pdd.Spec.TopologyHint.NodeSelector,
			Affinity:     pdd.Spec.TopologyHint.Affinity,
			Tolerations:  pdd.Spec.TopologyHint.Tolerations,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, pod, func() error {
		return ctrl.SetControllerReference(pdd, pod, r.Scheme)
	})

	if err == nil {
		pdd.Status.PrefillPodName = pod.Name
	}

	return err
}

// reconcileDecodePod creates or updates the decode pod
func (r *PrefillDecodingDisaggregationReconciler) reconcileDecodePod(ctx context.Context, pdd *productionstackv1alpha1.PrefillDecodingDisaggregation) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-decode", pdd.Name),
			Namespace: pdd.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "decode",
					Image: "vllm-decode:latest", // TODO: Make configurable
					Resources: corev1.ResourceRequirements{
						Requests: pdd.Spec.DecodeResources.Requests,
						Limits:   pdd.Spec.DecodeResources.Limits,
					},
				},
			},
			NodeSelector: pdd.Spec.TopologyHint.NodeSelector,
			Affinity:     pdd.Spec.TopologyHint.Affinity,
			Tolerations:  pdd.Spec.TopologyHint.Tolerations,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, pod, func() error {
		return ctrl.SetControllerReference(pdd, pod, r.Scheme)
	})

	if err == nil {
		pdd.Status.DecodePodName = pod.Name
	}

	return err
}

// reconcileService creates or updates the service
func (r *PrefillDecodingDisaggregationReconciler) reconcileService(ctx context.Context, pdd *productionstackv1alpha1.PrefillDecodingDisaggregation) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pdd.Name,
			Namespace: pdd.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "prefill",
					Port: 8000,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8000,
					},
				},
				{
					Name: "decode",
					Port: 8001,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8001,
					},
				},
			},
			Selector: map[string]string{
				"app": pdd.Name,
			},
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, service, func() error {
		return ctrl.SetControllerReference(pdd, service, r.Scheme)
	})

	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrefillDecodingDisaggregationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&productionstackv1alpha1.PrefillDecodingDisaggregation{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
