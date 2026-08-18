package webhook_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gloamers/openproject-bridge/internal/webhook"
)

func TestValidSignature(t *testing.T) {
	t.Parallel()

	secret := []byte("test-secret")
	body := []byte(`{"zen":"Design for failure."}`)
	sig := webhook.Sign(secret, body)

	if !webhook.ValidSignature(secret, body, sig) {
		t.Fatal("expected valid signature")
	}
	if webhook.ValidSignature(secret, body, "sha256=deadbeef") {
		t.Fatal("expected reject bad hex length / value")
	}
	if webhook.ValidSignature(secret, append(body, '!'), sig) {
		t.Fatal("expected reject mutated body")
	}
	if webhook.ValidSignature(nil, body, sig) {
		t.Fatal("expected reject empty secret")
	}
	if webhook.ValidSignature(secret, body, "") {
		t.Fatal("expected reject empty header")
	}
	if webhook.ValidSignature(secret, body, "sha1=abc") {
		t.Fatal("expected reject non-sha256 prefix")
	}
}

func TestHandlerPing(t *testing.T) {
	t.Parallel()

	secret := []byte("s3cret")
	body := []byte(`{"zen":"Non-blocking is better than blocking."}`)
	h := &webhook.Handler{Secret: secret}

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-GitHub-Delivery", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("X-Hub-Signature-256", webhook.Sign(secret, body))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp["status"] != "pong" {
		t.Fatalf("resp = %#v", resp)
	}
}

func TestHandlerRejectsBadSignature(t *testing.T) {
	t.Parallel()

	secret := []byte("s3cret")
	body := []byte(`{"action":"opened"}`)
	h := &webhook.Handler{Secret: secret}

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", webhook.Sign([]byte("wrong"), body))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandlerIssuesStubAccepted(t *testing.T) {
	t.Parallel()

	secret := []byte("s3cret")
	body := []byte(`{"action":"opened","issue":{"number":1}}`)
	h := &webhook.Handler{Secret: secret}

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-GitHub-Delivery", "22222222-2222-2222-2222-222222222222")
	req.Header.Set("X-Hub-Signature-256", webhook.Sign(secret, body))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerBodyTooLarge(t *testing.T) {
	t.Parallel()

	secret := []byte("s3cret")
	body := bytes.Repeat([]byte("a"), 64)
	h := &webhook.Handler{Secret: secret, MaxBody: 32}
	sigBody := body // signature over full body we send
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", webhook.Sign(secret, sigBody))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		b, _ := io.ReadAll(rr.Body)
		t.Fatalf("status = %d body = %s", rr.Code, b)
	}
}
