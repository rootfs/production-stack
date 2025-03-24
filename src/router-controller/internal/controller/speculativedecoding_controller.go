package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	productionstackv1alpha1 "github.com/vllm-project/production-stack/router-controller/api/v1alpha1"
)

// SpeculativeDecodingReconciler reconciles a SpeculativeDecoding object
type SpeculativeDecodingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=speculativedecodings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=speculativedecodings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=production-stack.vllm.ai,resources=speculativedecodings/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SpeculativeDecodingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	sd := &productionstackv1alpha1.SpeculativeDecoding{}
	if err := r.Get(ctx, req.NamespacedName, sd); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Reconcile draft pod
	if err := r.reconcileDraftPod(ctx, sd); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile service
	if err := r.reconcileService(ctx, sd); err != nil {
		return ctrl.Result{}, err
	}

	// Update status
	if err := r.Status().Update(ctx, sd); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileDraftPod creates or updates the draft model pod
func (r *SpeculativeDecodingReconciler) reconcileDraftPod(ctx context.Context, sd *productionstackv1alpha1.SpeculativeDecoding) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-draft", sd.Name),
			Namespace: sd.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "draft",
					Image: "vllm-draft:latest", // TODO: Make configurable
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"nvidia.com/gpu": resource.MustParse("1"),
							"memory":         resource.MustParse("16Gi"),
						},
						Limits: corev1.ResourceList{
							"nvidia.com/gpu": resource.MustParse("1"),
							"memory":         resource.MustParse("16Gi"),
						},
					},
					Args: []string{
						"--model", sd.Spec.DraftModel,
						"--host", "0.0.0.0",
						"--port", "8000",
					},
				},
			},
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, pod, func() error {
		return ctrl.SetControllerReference(sd, pod, r.Scheme)
	})

	if err == nil {
		sd.Status.DraftPodName = pod.Name
	}

	return err
}

// reconcileService creates or updates the service
func (r *SpeculativeDecodingReconciler) reconcileService(ctx context.Context, sd *productionstackv1alpha1.SpeculativeDecoding) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sd.Name,
			Namespace: sd.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "draft",
					Port: 8000,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8000,
					},
				},
			},
			Selector: map[string]string{
				"app": sd.Name,
			},
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, service, func() error {
		return ctrl.SetControllerReference(sd, service, r.Scheme)
	})

	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *SpeculativeDecodingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&productionstackv1alpha1.SpeculativeDecoding{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
