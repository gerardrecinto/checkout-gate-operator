package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheckoutGateSpec declares the SLO thresholds a target Deployment must
// stay under. Same five-signal shape as a plain CLI evaluator would use,
// expressed here as a real Kubernetes resource instead: error rate, p99
// latency, and resource saturation, checked by a controller that reconciles
// continuously instead of running once and exiting.
type CheckoutGateSpec struct {
	// TargetDeployment is the name of the Deployment this gate watches, in the same namespace as the CheckoutGate.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	TargetDeployment string `json:"targetDeployment"`

	// MaxErrorRateMilliPercent is the highest acceptable HTTP 5xx rate before the gate reports Breach,
	// expressed in thousandths of a percent (500 = 0.5%) so a sub-1% threshold stays an exact integer
	// instead of a float in a CRD schema, the same reason Kubernetes represents CPU in millicores.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100000
	MaxErrorRateMilliPercent int32 `json:"maxErrorRateMilliPercent"`

	// MaxP99LatencyMs is the highest acceptable p99 latency in milliseconds before the gate reports Breach.
	// +kubebuilder:validation:Minimum=0
	MaxP99LatencyMs int32 `json:"maxP99LatencyMs"`

	// MaxCpuSaturationPercent is the highest acceptable average CPU saturation across the target's pods before the gate reports Warn.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	MaxCpuSaturationPercent int32 `json:"maxCpuSaturationPercent"`
}

// GateVerdict is the outcome of the most recent reconciliation.
type GateVerdict string

const (
	VerdictUnknown GateVerdict = "Unknown"
	VerdictPass    GateVerdict = "Pass"
	VerdictWarn    GateVerdict = "Warn"
	VerdictBreach  GateVerdict = "Breach"
)

// CheckoutGateStatus is written by the controller on every reconcile, never by a user.
type CheckoutGateStatus struct {
	// Verdict is the outcome of the most recent evaluation.
	Verdict GateVerdict `json:"verdict,omitempty"`

	// Message is a short, human-readable explanation of the verdict.
	Message string `json:"message,omitempty"`

	// ObservedGeneration lets a client tell whether Status reflects the most recent Spec edit.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastEvaluated is when this Status was last written.
	LastEvaluated metav1.Time `json:"lastEvaluated,omitempty"`

	// ReadyReplicas mirrors the target Deployment's ready replica count at evaluation time.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetDeployment`
// +kubebuilder:printcolumn:name="Verdict",type=string,JSONPath=`.status.verdict`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CheckoutGate is the Schema for the checkoutgates API.
type CheckoutGate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheckoutGateSpec   `json:"spec,omitempty"`
	Status CheckoutGateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CheckoutGateList contains a list of CheckoutGate.
type CheckoutGateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CheckoutGate `json:"items"`
}
