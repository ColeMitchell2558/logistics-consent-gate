package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"example.com/logistics-consent-gate/internal/consent"
	"example.com/logistics-consent-gate/internal/shipment"
)

type server struct {
	consent *consent.Client
	log     *slog.Logger
}

type consentChange struct {
	Category       string `json:"category"`
	Source         string `json:"source"`
	IdempotencyKey string `json:"idempotency_key"`
}

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "INFRAI_API_KEY is required")
		os.Exit(2)
	}
	address := os.Getenv("LISTEN_ADDR")
	if address == "" {
		address = ":8080"
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := &server{consent: consent.NewClient(apiKey), log: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shipment-events", s.shipmentEvent)
	mux.HandleFunc("PUT /users/{user_id}/consents", s.grant)
	mux.HandleFunc("DELETE /users/{user_id}/consents", s.revoke)

	httpServer := &http.Server{Addr: address, Handler: requestLog(logger, mux), ReadHeaderTimeout: 5 * time.Second}
	logger.Info("listening", "address", address)
	if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func (s *server) shipmentEvent(w http.ResponseWriter, r *http.Request) {
	var event shipment.Event
	if err := decodeJSON(r, &event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	scope, err := shipment.RequiredScope(event)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	granted, err := s.consent.Check(r.Context(), event.UserID, scope)
	if err != nil {
		s.writeConsentError(w, err)
		return
	}
	decision, _ := shipment.Decide(event, granted)
	status := http.StatusAccepted
	if !decision.Accepted {
		status = http.StatusForbidden
	}
	s.log.Info("shipment consent decision", "shipment_id", event.ShipmentID, "scope", scope, "accepted", decision.Accepted)
	writeJSON(w, status, decision)
}

func (s *server) grant(w http.ResponseWriter, r *http.Request) {
	var change consentChange
	if err := decodeJSON(r, &change); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if change.Category == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category is required"})
		return
	}
	if err := s.consent.Grant(r.Context(), r.PathValue("user_id"), change.Category, change.Source, change.IdempotencyKey); err != nil {
		s.writeConsentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "granted", "category": change.Category})
}

func (s *server) revoke(w http.ResponseWriter, r *http.Request) {
	var change consentChange
	if err := decodeJSON(r, &change); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if change.Category == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category is required"})
		return
	}
	if err := s.consent.Revoke(r.Context(), r.PathValue("user_id"), change.Category, change.IdempotencyKey); err != nil {
		s.writeConsentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked", "category": change.Category})
}

func (s *server) writeConsentError(w http.ResponseWriter, err error) {
	var apiErr *consent.APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		writeJSON(w, apiErr.Status, map[string]string{"error": apiErr.Message, "code": apiErr.Code})
		return
	}
	s.log.Error("consent dependency error", "error", err)
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "consent dependency unavailable"})
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", strings.TrimSpace(r.URL.Path), "duration_ms", time.Since(started).Milliseconds())
	})
}
