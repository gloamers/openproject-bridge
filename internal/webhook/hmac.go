package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const signaturePrefix = "sha256="

// ValidSignature checks X-Hub-Signature-256 against HMAC-SHA256(secret, body).
// Comparison is constant-time. Empty secret or header → false.
func ValidSignature(secret, body []byte, header string) bool {
	if len(secret) == 0 || header == "" {
		return false
	}
	got, ok := parseSignature(header)
	if !ok {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	want := mac.Sum(nil)
	return hmac.Equal(got, want)
}

func parseSignature(header string) ([]byte, bool) {
	if !strings.HasPrefix(header, signaturePrefix) {
		return nil, false
	}
	sum, err := hex.DecodeString(strings.TrimPrefix(header, signaturePrefix))
	if err != nil || len(sum) != sha256.Size {
		return nil, false
	}
	return sum, true
}

// Sign returns the X-Hub-Signature-256 value for tests and local tooling.
func Sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}
