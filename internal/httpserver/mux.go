package httpserver

import (
	"encoding/json"
	"net/http"
)

// Options for built-in routes.
type Options struct {
	// WebhookPath is the GitHub webhook endpoint (default /webhooks/github).
	WebhookPath string
	// Webhook is the handler for GitHub deliveries. If nil, a stub 501 is mounted.
	Webhook http.Handler
}

func (o Options) withDefaults() Options {
	if o.WebhookPath == "" {
		o.WebhookPath = "/webhooks/github"
	}
	return o
}

// NewMux returns a ServeMux with health and webhook routes.
func NewMux(opts Options) *http.ServeMux {
	opts = opts.withDefaults()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	wh := opts.Webhook
	if wh == nil {
		wh = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusNotImplemented, map[string]string{
				"error": "webhook handler not configured",
			})
		})
	}
	mux.Handle("POST "+opts.WebhookPath, wh)

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
