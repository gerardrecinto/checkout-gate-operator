// Package v1alpha1 contains the CheckoutGate CRD types and their scheme registration.
// +kubebuilder:object:generate=true
// +groupName=checkout.gerardrecinto.dev
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion  = schema.GroupVersion{Group: "checkout.gerardrecinto.dev", Version: "v1alpha1"}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	AddToScheme   = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(&CheckoutGate{}, &CheckoutGateList{})
}
