// Package controller holds the CheckoutGate reconciler.
package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	checkoutv1alpha1 "github.com/gerardrecinto/checkout-gate-operator/api/v1alpha1"
	"github.com/gerardrecinto/checkout-gate-operator/internal/gate"
)

// MetricsProvider is the seam between the controller and wherever real
// numbers come from (Prometheus in production, a fake in tests). Same
// interface-at-the-boundary pattern used throughout this session's other
// repos: the reconciler doesn't know or care what's behind it.
type MetricsProvider interface {
	FetchMetrics(ctx context.Context, namespace, deploymentName string) (gate.Metrics, error)
}

// CheckoutGateReconciler reconciles a CheckoutGate object.
type CheckoutGateReconciler struct {
	client.Client
	Metrics MetricsProvider
}

// +kubebuilder:rbac:groups=checkout.gerardrecinto.dev,resources=checkoutgates,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=checkout.gerardrecinto.dev,resources=checkoutgates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch

func (r *CheckoutGateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var cg checkoutv1alpha1.CheckoutGate
	if err := r.Get(ctx, req.NamespacedName, &cg); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching CheckoutGate: %w", err)
	}

	var dep appsv1.Deployment
	depKey := types.NamespacedName{Namespace: cg.Namespace, Name: cg.Spec.TargetDeployment}
	if err := r.Get(ctx, depKey, &dep); err != nil {
		if apierrors.IsNotFound(err) {
			cg.Status.Verdict = checkoutv1alpha1.VerdictUnknown
			cg.Status.Message = fmt.Sprintf("target deployment %q not found", cg.Spec.TargetDeployment)
			cg.Status.LastEvaluated = metav1.Now()
			cg.Status.ObservedGeneration = cg.Generation
			if updErr := r.Status().Update(ctx, &cg); updErr != nil {
				return ctrl.Result{}, updErr
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching target Deployment: %w", err)
	}

	metrics, err := r.Metrics.FetchMetrics(ctx, cg.Namespace, cg.Spec.TargetDeployment)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("fetching metrics: %w", err)
	}
	metrics.ReadyReplicas = dep.Status.ReadyReplicas
	if dep.Spec.Replicas != nil {
		metrics.DesiredReplicas = *dep.Spec.Replicas
	}

	result := gate.Evaluate(cg.Spec, metrics)

	cg.Status.Verdict = result.Verdict
	cg.Status.Message = result.Message
	cg.Status.ReadyReplicas = dep.Status.ReadyReplicas
	cg.Status.LastEvaluated = metav1.Now()
	cg.Status.ObservedGeneration = cg.Generation

	if err := r.Status().Update(ctx, &cg); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating CheckoutGate status: %w", err)
	}

	logger.Info("reconciled CheckoutGate", "name", cg.Name, "verdict", result.Verdict, "message", result.Message)

	return ctrl.Result{}, nil
}

// mapDeploymentToGates finds every CheckoutGate in the changed Deployment's
// namespace that references it by name, so an edit to a plain Deployment
// (a rollout, a replica change) re-triggers reconciliation of the gates
// watching it. A CheckoutGate doesn't own its target Deployment, it
// references it by name, so `.Owns()` (which follows OwnerReferences) is
// the wrong tool here; this indexed lookup is the correct one.
func (r *CheckoutGateReconciler) mapDeploymentToGates(ctx context.Context, obj client.Object) []ctrl.Request {
	dep, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil
	}

	var gates checkoutv1alpha1.CheckoutGateList
	if err := r.List(ctx, &gates, client.InNamespace(dep.Namespace)); err != nil {
		return nil
	}

	var requests []ctrl.Request
	for _, cg := range gates.Items {
		if cg.Spec.TargetDeployment == dep.Name {
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{Namespace: cg.Namespace, Name: cg.Name},
			})
		}
	}
	return requests
}

func (r *CheckoutGateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&checkoutv1alpha1.CheckoutGate{}).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(r.mapDeploymentToGates),
		).
		Complete(r)
}
