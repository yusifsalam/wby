package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	signatureHeaderClientID  = "X-Client-ID"
	signatureHeaderTimestamp = "X-Timestamp"
	signatureHeaderValue     = "X-Signature"
)

func NewRequestSignatureMiddleware(clientSecrets map[string]string, maxAge time.Duration) func(http.Handler) http.Handler {
	secretByClient := make(map[string][]byte, len(clientSecrets))
	for clientID, secret := range clientSecrets {
		cleanClientID := strings.TrimSpace(clientID)
		cleanSecret := strings.TrimSpace(secret)
		if cleanClientID == "" || cleanSecret == "" {
			continue
		}
		secretByClient[cleanClientID] = []byte(cleanSecret)
	}
	if maxAge <= 0 {
		maxAge = 5 * time.Minute
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/v1/") {
				next.ServeHTTP(w, r)
				return
			}

			clientID := strings.TrimSpace(r.Header.Get(signatureHeaderClientID))
			timestamp := strings.TrimSpace(r.Header.Get(signatureHeaderTimestamp))
			signature := strings.TrimSpace(r.Header.Get(signatureHeaderValue))
			signature = strings.TrimPrefix(signature, "sha256=")

			if clientID == "" || timestamp == "" || signature == "" {
				writeJSONError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			secret, ok := secretByClient[clientID]
			if !ok {
				writeJSONError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if !isFreshTimestamp(timestamp, maxAge, time.Now()) {
				writeJSONError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			signatureBytes, err := hex.DecodeString(signature)
			if err != nil {
				writeJSONError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			expected := buildSignature(secret, r.Method, r.URL.Path, r.URL.RawQuery, timestamp)
			if !hmac.Equal(signatureBytes, expected) {
				writeJSONError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isFreshTimestamp(ts string, maxAge time.Duration, now time.Time) bool {
	epochSeconds, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	reqTime := time.Unix(epochSeconds, 0)
	age := now.Sub(reqTime)
	if age < 0 {
		age = -age
	}
	return age <= maxAge
}

func buildSignature(secret []byte, method, path, rawQuery, timestamp string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(method))
	mac.Write([]byte("\n"))
	mac.Write([]byte(path))
	mac.Write([]byte("\n"))
	mac.Write([]byte(rawQuery))
	mac.Write([]byte("\n"))
	mac.Write([]byte(timestamp))
	return mac.Sum(nil)
}
