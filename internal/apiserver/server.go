// Package apiserver exposes CheckoutGate status as JSON for the frontend.
// Implemented as a controller-runtime manager.Runnable rather than a
// separate process: it shares the manager's cache (a controller-runtime
// client.Reader backed by informers already watching CheckoutGate), so a
// list call here is a cache read, not a fresh API server round-trip.
package apiserver

import (
	"context"
	"encoding/json"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"

	checkoutv1alpha1 "github.com/gerardrecinto/checkout-gate-operator/api/v1alpha1"
)

type GateSummary struct {
	Name             string `json:"name"`
	Namespace        string `json:"namespace"`
	TargetDeployment string `json:"targetDeployment"`
	Verdict          string `json:"verdict"`
	Message          string `json:"message"`
	ReadyReplicas    int32  `json:"readyReplicas"`
	LastEvaluated    string `json:"lastEvaluated,omitempty"`
}

// VerdictCounts tallies gates by verdict, for a dashboard header stat that
// shouldn't have to fetch and count the full gate list itself.
type VerdictCounts struct {
	Total   int `json:"total"`
	Pass    int `json:"pass"`
	Warn    int `json:"warn"`
	Breach  int `json:"breach"`
	Unknown int `json:"unknown"`
}

// Server implements manager.Runnable and manager.LeaderElectionRunnable:
// it only needs to run on the leader replica, same as the controller
// itself, two Runnables sharing one leader election decision instead of
// each managing its own.
type Server struct {
	Reader client.Reader
	Addr   string
}

var _ ctrlmgr.Runnable = &Server{}
var _ ctrlmgr.LeaderElectionRunnable = &Server{}

func (s *Server) NeedLeaderElection() bool { return true }

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/gates", s.handleListGates)
	mux.HandleFunc("/api/gates/summary", s.handleSummary)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	srv := &http.Server{Addr: s.Addr, Handler: withCORS(mux)}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) handleListGates(w http.ResponseWriter, r *http.Request) {
	var list checkoutv1alpha1.CheckoutGateList
	if err := s.Reader.List(r.Context(), &list); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summaries := make([]GateSummary, 0, len(list.Items))
	for _, cg := range list.Items {
		summary := GateSummary{
			Name:             cg.Name,
			Namespace:        cg.Namespace,
			TargetDeployment: cg.Spec.TargetDeployment,
			Verdict:          string(cg.Status.Verdict),
			Message:          cg.Status.Message,
			ReadyReplicas:    cg.Status.ReadyReplicas,
		}
		if !cg.Status.LastEvaluated.IsZero() {
			summary.LastEvaluated = cg.Status.LastEvaluated.Format("2006-01-02T15:04:05Z07:00")
		}
		summaries = append(summaries, summary)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(summaries); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	var list checkoutv1alpha1.CheckoutGateList
	if err := s.Reader.List(r.Context(), &list); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var counts VerdictCounts
	for _, cg := range list.Items {
		counts.Total++
		switch cg.Status.Verdict {
		case checkoutv1alpha1.VerdictPass:
			counts.Pass++
		case checkoutv1alpha1.VerdictWarn:
			counts.Warn++
		case checkoutv1alpha1.VerdictBreach:
			counts.Breach++
		default:
			counts.Unknown++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(counts); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// withCORS allows the frontend dev server (a different origin during
// local development, npm run dev on its own port) to call this API
// directly instead of needing a proxy config just to develop the UI.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
