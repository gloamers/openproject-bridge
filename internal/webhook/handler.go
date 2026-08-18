package webhook

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/defaults"
	"github.com/gloamers/openproject-bridge/internal/domain"
)

const (
	headerSignature = "X-Hub-Signature-256"
	headerEvent     = "X-GitHub-Event"
	headerDelivery  = "X-GitHub-Delivery"
)

// Handler verifies GitHub HMAC and processes / enqueues deliveries.
type Handler struct {
	Root       *config.Root
	Deliveries DeliveryRepository
	Sync       IssueSync
	Secret     []byte // fallback when Root is nil (tests / simple mode)
	MaxBody    int64
	Log        *slog.Logger

	queue    chan job
	once     sync.Once
	wg       sync.WaitGroup
	inflight sync.WaitGroup
	closed   atomic.Bool
}

type job struct {
	d domain.Delivery
}

func (h *Handler) maxBody() int64 {
	if h.MaxBody > 0 {
		return h.MaxBody
	}
	if h.Root != nil && h.Root.Bridge.Webhook.MaxBodyBytes > 0 {
		return h.Root.Bridge.Webhook.MaxBodyBytes
	}
	return defaults.MaxBodyBytes
}

func (h *Handler) log() *slog.Logger {
	if h.Log != nil {
		return h.Log
	}
	return slog.Default()
}

// StartWorkers starts background processors (call once).
func (h *Handler) StartWorkers(n int) {
	h.once.Do(func() {
		if n <= 0 {
			n = defaults.WebhookWorkers
		}
		h.queue = make(chan job, defaults.WebhookQueue)
		for i := 0; i < n; i++ {
			h.wg.Add(1)
			go h.worker()
		}
	})
}

// ShutdownWorkers stops accepting, waits for in-flight handlers, then drains workers.
func (h *Handler) ShutdownWorkers() {
	h.closed.Store(true)
	h.inflight.Wait()
	if h.queue != nil {
		close(h.queue)
		h.wg.Wait()
	}
}

func (h *Handler) worker() {
	defer h.wg.Done()
	for j := range h.queue {
		h.process(context.Background(), j.d)
	}
}

func (h *Handler) process(ctx context.Context, d domain.Delivery) {
	if h.Sync == nil {
		return
	}
	err := h.Sync.Handle(ctx, d)
	if h.Deliveries == nil || d.DeliveryID == "" {
		if err != nil {
			h.log().Error("sync failed", slog.String("err", err.Error()))
		}
		return
	}
	if err != nil {
		h.log().Error("sync failed",
			slog.String("event", d.Event),
			slog.String("delivery", d.DeliveryID),
			slog.String("err", err.Error()),
		)
		_ = h.Deliveries.FailDelivery(ctx, d.DeliveryID)
		return
	}
	if err := h.Deliveries.CompleteDelivery(ctx, d.DeliveryID); err != nil {
		h.log().Error("complete delivery failed", slog.String("err", err.Error()))
	}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.closed.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "shutting down"})
		return
	}
	h.inflight.Add(1)
	defer h.inflight.Done()

	log := h.log().With(
		slog.String("op", "webhook.github"),
		slog.String("event", r.Header.Get(headerEvent)),
		slog.String("delivery", r.Header.Get(headerDelivery)),
	)

	limit := h.maxBody()
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		log.Warn("read body failed", slog.String("err", err.Error()))
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if int64(len(body)) > limit {
		log.Warn("body too large", slog.Int64("limit", limit))
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "body too large"})
		return
	}

	secret, err := h.resolveSecret(body)
	if err != nil || !ValidSignature(secret, body, r.Header.Get(headerSignature)) {
		log.Warn("signature rejected")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
		return
	}

	event := r.Header.Get(headerEvent)
	deliveryID := r.Header.Get(headerDelivery)

	switch event {
	case "ping":
		log.Info("ping accepted")
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "event": event, "status": "pong"})
		return
	case "":
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing X-GitHub-Event"})
		return
	}

	d := domain.Delivery{Event: event, DeliveryID: deliveryID, Body: body}

	if h.Deliveries != nil && deliveryID != "" {
		started, alreadyDone, err := h.Deliveries.BeginDelivery(r.Context(), deliveryID)
		if err != nil {
			log.Error("begin delivery failed", slog.String("err", err.Error()))
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
			return
		}
		if alreadyDone {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "duplicate"})
			return
		}
		if !started {
			// Another in-flight processing owns this delivery id.
			writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "status": "in_flight"})
			return
		}
	}

	if h.queue != nil && h.Sync != nil {
		if h.closed.Load() {
			if h.Deliveries != nil {
				_ = h.Deliveries.FailDelivery(r.Context(), deliveryID)
			}
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "shutting down"})
			return
		}
		select {
		case h.queue <- job{d: d}:
			writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "event": event, "status": "accepted"})
		default:
			// Queue full: process inline.
			h.process(r.Context(), d)
			writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "event": event, "status": "accepted"})
		}
		return
	}

	if h.Sync != nil {
		h.process(r.Context(), d)
	} else if h.Deliveries != nil && deliveryID != "" {
		_ = h.Deliveries.CompleteDelivery(r.Context(), deliveryID)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "event": event, "status": "accepted"})
}

func (h *Handler) resolveSecret(body []byte) ([]byte, error) {
	if h.Root == nil {
		if len(h.Secret) == 0 {
			return nil, errNoSecret
		}
		return h.Secret, nil
	}
	var peek struct {
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	_ = json.Unmarshal(body, &peek)
	if full := strings.TrimSpace(peek.Repository.FullName); full != "" {
		if route, err := h.Root.ResolveGitHub(full); err == nil {
			s, err := h.Root.ResolveWebhookSecret(route.Org)
			if err != nil {
				return nil, err
			}
			return []byte(s), nil
		}
	}
	// Unmapped repo (or ping without useful body): bridge fallback / test Secret.
	s, err := h.Root.FallbackWebhookSecret()
	if err != nil {
		if len(h.Secret) > 0 {
			return h.Secret, nil
		}
		return nil, err
	}
	return []byte(s), nil
}

var errNoSecret = errString("webhook: no secret configured")

type errString string

func (e errString) Error() string { return string(e) }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
