package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRequestSignatureMiddleware_AllowsValidSignedRequest(t *testing.T) {
	clientID := "ios-app"
	secret := "top-secret"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req := httptest.NewRequest(http.MethodGet, "/v1/weather?lat=60.1&lon=24.9", nil)
	req.Header.Set("X-Client-ID", clientID)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Signature", signForTest(secret, req.Method, req.URL.Path, req.URL.RawQuery, ts))

	rr := httptest.NewRecorder()
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	middleware := NewRequestSignatureMiddleware(map[string]string{clientID: secret}, 5*time.Minute)
	middleware(next).ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatalf("expected next handler to be called")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rr.Code)
	}
}

func TestRequestSignatureMiddleware_RejectsUnsignedRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/weather?lat=60.1&lon=24.9", nil)
	rr := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	middleware := NewRequestSignatureMiddleware(map[string]string{"ios-app": "top-secret"}, 5*time.Minute)
	middleware(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestRequestSignatureMiddleware_RejectsUnknownClient(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req := httptest.NewRequest(http.MethodGet, "/v1/weather?lat=60.1&lon=24.9", nil)
	req.Header.Set("X-Client-ID", "unknown")
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Signature", signForTest("wrong-secret", req.Method, req.URL.Path, req.URL.RawQuery, ts))

	rr := httptest.NewRecorder()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	middleware := NewRequestSignatureMiddleware(map[string]string{"ios-app": "top-secret"}, 5*time.Minute)
	middleware(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestRequestSignatureMiddleware_RejectsStaleTimestamp(t *testing.T) {
	clientID := "ios-app"
	secret := "top-secret"
	ts := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	req := httptest.NewRequest(http.MethodGet, "/v1/weather?lat=60.1&lon=24.9", nil)
	req.Header.Set("X-Client-ID", clientID)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Signature", signForTest(secret, req.Method, req.URL.Path, req.URL.RawQuery, ts))

	rr := httptest.NewRecorder()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	middleware := NewRequestSignatureMiddleware(map[string]string{clientID: secret}, 5*time.Minute)
	middleware(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestRequestSignatureMiddleware_BypassesNonAPIRoutes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	middleware := NewRequestSignatureMiddleware(map[string]string{"ios-app": "top-secret"}, 5*time.Minute)
	middleware(next).ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatalf("expected next handler to be called")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rr.Code)
	}
}

func signForTest(secret, method, path, rawQuery, ts string) string {
	msg := method + "\n" + path + "\n" + rawQuery + "\n" + ts
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}
