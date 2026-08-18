package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gloamers/openproject-bridge/internal/httpserver"
	"github.com/gloamers/openproject-bridge/internal/webhook"
)

func TestStartShutdownHealthz(t *testing.T) {
	t.Parallel()

	mux := httpserver.NewMux(httpserver.Options{})
	srv := httpserver.New(httpserver.Config{
		Addr:            "127.0.0.1:0",
		ShutdownTimeout: 2 * time.Second,
	}, mux)

	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	base := "http://" + srv.Addr()

	res, err := http.Get(base + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body = %#v", body)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	res2, err := http.Get(base + "/healthz")
	if err == nil {
		res2.Body.Close()
		t.Fatal("expected connection error after shutdown")
	}
}

func TestRunCancelsAndShutsDown(t *testing.T) {
	t.Parallel()

	mux := httpserver.NewMux(httpserver.Options{})
	srv := httpserver.New(httpserver.Config{
		Addr:            "127.0.0.1:0",
		ShutdownTimeout: 2 * time.Second,
	}, mux)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	// Wait until listening.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if addr := srv.Addr(); addr != "127.0.0.1:0" && addr != "" {
			res, err := http.Get("http://" + addr + "/readyz")
			if err == nil {
				res.Body.Close()
				if res.StatusCode == http.StatusOK {
					break
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestWebhookHMAC(t *testing.T) {
	t.Parallel()

	secret := []byte("integration-secret")
	mux := httpserver.NewMux(httpserver.Options{
		Webhook: &webhook.Handler{Secret: secret},
	})
	srv := httpserver.New(httpserver.Config{Addr: "127.0.0.1:0"}, mux)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	body := []byte(`{"zen":"Responsive is better than fast."}`)
	req, err := http.NewRequest(http.MethodPost, "http://"+srv.Addr()+"/webhooks/github", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", webhook.Sign(secret, body))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d body = %s", res.StatusCode, b)
	}
}
