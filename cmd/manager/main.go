// Command manager runs the checkout-gate-operator: the CheckoutGate
// controller, its validating webhook, and the status API the frontend
// reads from, all inside one controller-runtime manager.
//
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	checkoutv1alpha1 "github.com/gerardrecinto/checkout-gate-operator/api/v1alpha1"
	"github.com/gerardrecinto/checkout-gate-operator/internal/apiserver"
	"github.com/gerardrecinto/checkout-gate-operator/internal/controller"
	"github.com/gerardrecinto/checkout-gate-operator/internal/notify"
	internalwebhook "github.com/gerardrecinto/checkout-gate-operator/internal/webhook"
)

var scheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = checkoutv1alpha1.AddToScheme(scheme)
}

func main() {
	var metricsAddr, apiAddr, prometheusAddr, webhookCertDir, natsURL string
	var enableLeaderElection bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8443", "Address the metrics endpoint binds to.")
	flag.StringVar(&apiAddr, "api-bind-address", ":8081", "Address the status API for the frontend binds to.")
	flag.StringVar(&prometheusAddr, "prometheus-address", "http://prometheus-k8s.monitoring.svc:9090", "Prometheus base URL used to evaluate gates.")
	flag.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs", "Directory holding the webhook's TLS cert/key, provisioned by cert-manager in production, see config/webhook.")
	flag.StringVar(&natsURL, "nats-url", "", "NATS server URL to publish CheckoutGate verdict transitions to (e.g. nats://nats.messaging.svc:4222). Empty disables notifications entirely and uses a no-op notifier.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election, so only one replica reconciles at a time.")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		WebhookServer:          webhook.NewServer(webhook.Options{CertDir: webhookCertDir}),
		HealthProbeBindAddress: ":8082",
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "checkout-gate-operator.gerardrecinto.dev",
	})
	if err != nil {
		ctrl.Log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	promMetrics, err := controller.NewPrometheusMetricsProvider(prometheusAddr)
	if err != nil {
		ctrl.Log.Error(err, "unable to create Prometheus metrics provider")
		os.Exit(1)
	}

	var verdictNotifier notify.Notifier = notify.NoopNotifier{}
	if natsURL != "" {
		natsNotifier, err := notify.NewNATSNotifier(natsURL)
		if err != nil {
			ctrl.Log.Error(err, "unable to connect to NATS")
			os.Exit(1)
		}
		defer natsNotifier.Close()
		verdictNotifier = natsNotifier
	}

	if err := (&controller.CheckoutGateReconciler{
		Client:   mgr.GetClient(),
		Metrics:  promMetrics,
		Notifier: verdictNotifier,
	}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to set up CheckoutGate controller")
		os.Exit(1)
	}

	if err := (&internalwebhook.CheckoutGateValidator{}).SetupWebhookWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to set up CheckoutGate webhook")
		os.Exit(1)
	}

	if err := mgr.Add(&apiserver.Server{Reader: mgr.GetClient(), Addr: apiAddr}); err != nil {
		ctrl.Log.Error(err, "unable to register status API server")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	ctrl.Log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
