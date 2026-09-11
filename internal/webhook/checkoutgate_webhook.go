// Package webhook holds the CheckoutGate validating admission webhook.
// This is deliberately kept separate from what the CRD's OpenAPI schema
// already enforces (kubebuilder validation markers on the Spec fields,
// see api/v1alpha1/checkoutgate_types.go): a webhook is for checks that
// need a live API call the schema can't express on its own, here, that
// the referenced target Deployment actually exists in the same namespace.
package webhook

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	checkoutv1alpha1 "github.com/gerardrecinto/checkout-gate-operator/api/v1alpha1"
)

// +kubebuilder:webhook:path=/validate-checkout-gerardrecinto-dev-v1alpha1-checkoutgate,mutating=false,failurePolicy=fail,sideEffects=None,groups=checkout.gerardrecinto.dev,resources=checkoutgates,verbs=create;update,versions=v1alpha1,name=vcheckoutgate.gerardrecinto.dev,admissionReviewVersions=v1

// CheckoutGateValidator implements admission.CustomValidator for CheckoutGate.
type CheckoutGateValidator struct {
	Client client.Reader
}

var _ admission.CustomValidator = &CheckoutGateValidator{}

func (v *CheckoutGateValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	cg, ok := obj.(*checkoutv1alpha1.CheckoutGate)
	if !ok {
		return nil, fmt.Errorf("expected a CheckoutGate, got %T", obj)
	}
	return v.validate(ctx, cg)
}

func (v *CheckoutGateValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	cg, ok := newObj.(*checkoutv1alpha1.CheckoutGate)
	if !ok {
		return nil, fmt.Errorf("expected a CheckoutGate, got %T", newObj)
	}
	return v.validate(ctx, cg)
}

func (v *CheckoutGateValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *CheckoutGateValidator) validate(ctx context.Context, cg *checkoutv1alpha1.CheckoutGate) (admission.Warnings, error) {
	var warnings admission.Warnings

	var dep appsv1.Deployment
	key := types.NamespacedName{Namespace: cg.Namespace, Name: cg.Spec.TargetDeployment}
	err := v.Client.Get(ctx, key, &dep)
	if apierrors.IsNotFound(err) {
		// A warning, not a hard rejection: a CheckoutGate applied ahead of its
		// target Deployment (a common GitOps ordering, both manifests land in
		// the same apply) is a real, valid state, the controller reports
		// Unknown until the Deployment shows up. Rejecting outright would
		// break that ordering for no safety benefit.
		warnings = append(warnings, fmt.Sprintf(
			"target Deployment %q does not exist yet in namespace %q; this gate will report Unknown until it does",
			cg.Spec.TargetDeployment, cg.Namespace,
		))
	} else if err != nil {
		return warnings, fmt.Errorf("checking target Deployment: %w", err)
	}

	if cg.Spec.MaxP99LatencyMs == 0 {
		return warnings, fmt.Errorf("maxP99LatencyMs must be greater than 0, a zero threshold breaches on any real traffic")
	}

	return warnings, nil
}

// SetupWebhookWithManager registers this validator with the manager's webhook server.
func (v *CheckoutGateValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&checkoutv1alpha1.CheckoutGate{}).
		WithValidator(v).
		Complete()
}
