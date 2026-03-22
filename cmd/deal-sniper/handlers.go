package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/madfam-org/server-auction-tracker/internal/store"
	log "github.com/sirupsen/logrus"
)

// scanRecordToServer converts a ScanRecord back into a scanner.Server
// with enough data for the simulate package.
func scanRecordToServer(r *store.ScanRecord) scanner.Server {
	return scanner.Server{
		ID:             r.ServerID,
		CPU:            r.CPU,
		RAMSize:        r.RAMSize,
		TotalStorageTB: r.TotalStorageTB,
		NVMeCount:      r.NVMeCount,
		DriveCount:     r.DriveCount,
		Datacenter:     r.Datacenter,
		Price:          r.Price,
		ParsedCores:    estimateCoresFromRAM(r.RAMSize),
	}
}

// estimateCoresFromRAM provides a rough core estimate when we only have scan records.
// Hetzner servers with 128GB+ typically have 16+ cores.
func estimateCoresFromRAM(ramGB int) int {
	switch {
	case ramGB >= 256:
		return 32
	case ramGB >= 128:
		return 16
	case ramGB >= 64:
		return 8
	default:
		return 4
	}
}

// --- Auth middleware ---

// authMiddleware validates requests using a three-tier strategy:
//  1. Janua JWT from ds_session cookie (browser SSO)
//  2. Bearer token validated as Janua JWT
//  3. Bearer token matching static DEAL_SNIPER_AUTH_TOKEN env var (backward compat)
//
// If none succeed, returns 401. If no auth is configured at all, returns 503.
func (s *server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Try Janua JWT (cookie or Bearer) if SSO is configured
		if s.validator != nil && s.oauth != nil {
			tokenStr := s.oauth.ExtractToken(r)
			if tokenStr != "" {
				claims, err := s.validator.ValidateToken(tokenStr)
				if err == nil && s.validator.IsAuthorized(claims) {
					next(w, r)
					return
				}
				// Token was present but invalid — don't fall through to static token
				// unless the token looks like it could be the static token
				if !isStaticTokenCandidate(tokenStr) {
					writeJSON(w, http.StatusUnauthorized, map[string]string{
						"error": "invalid or expired JWT",
					})
					return
				}
			}
		}

		// 2. Fall back to static DEAL_SNIPER_AUTH_TOKEN
		staticToken := os.Getenv("DEAL_SNIPER_AUTH_TOKEN")
		if staticToken == "" && s.validator == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "order API is not configured (no auth token set)",
			})
			return
		}

		if staticToken != "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") && strings.TrimPrefix(authHeader, "Bearer ") == staticToken {
				next(w, r)
				return
			}
		}

		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "invalid or missing authorization token",
		})
	}
}

// isStaticTokenCandidate returns true if the token does NOT look like a JWT.
// JWTs have three dot-separated base64 segments; static tokens typically don't.
func isStaticTokenCandidate(token string) bool {
	return strings.Count(token, ".") < 2
}

// --- Order handlers ---

type orderCheckRequest struct {
	ServerID int `json:"server_id"`
}

type orderCheckResponse struct {
	Eligible  bool             `json:"eligible"`
	Reasons   []string         `json:"reasons,omitempty"`
	ServerID  int              `json:"server_id"`
	Score     float64          `json:"score"`
	Price     float64          `json:"price"`
	CPU       string           `json:"cpu"`
	Breakdown *scorer.Breakdown `json:"breakdown,omitempty"`
}

type orderConfirmResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	TransactionID string `json:"transaction_id,omitempty"`
}

// handleOrderCheck performs an eligibility pre-check without placing an order.
func (s *server) handleOrderCheck(w http.ResponseWriter, r *http.Request) {
	var req orderCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.ServerID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "server_id is required"})
		return
	}

	srv, score, breakdown, err := s.fetchAndScoreServer(req.ServerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	check := s.orderer.CheckEligibility(srv, score, &s.config.Order)

	writeJSON(w, http.StatusOK, orderCheckResponse{
		Eligible:  check.Eligible,
		Reasons:   check.Reasons,
		ServerID:  srv.ID,
		Score:     score,
		Price:     srv.Price,
		CPU:       srv.CPU,
		Breakdown: breakdown,
	})
}

// handleOrderConfirm places an order after re-checking eligibility.
func (s *server) handleOrderConfirm(w http.ResponseWriter, r *http.Request) {
	var req orderCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.ServerID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "server_id is required"})
		return
	}

	srv, score, _, err := s.fetchAndScoreServer(req.ServerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	check := s.orderer.CheckEligibility(srv, score, &s.config.Order)
	if !check.Eligible {
		writeJSON(w, http.StatusOK, orderConfirmResponse{
			Success: false,
			Message: fmt.Sprintf("ineligible: %s", strings.Join(check.Reasons, "; ")),
		})
		return
	}

	result, err := s.orderer.Order(r.Context(), srv.ID)
	if err != nil {
		log.WithError(err).WithField("server_id", srv.ID).Error("Order API call failed")
		writeJSON(w, http.StatusInternalServerError, orderConfirmResponse{
			Success: false,
			Message: fmt.Sprintf("order failed: %v", err),
		})
		return
	}

	// Save audit record
	if saveErr := s.store.SaveOrderAttempt(srv.ID, score, srv.Price, result.Success, result.Message); saveErr != nil {
		log.WithError(saveErr).Error("Failed to save order attempt")
	}

	writeJSON(w, http.StatusOK, orderConfirmResponse{
		Success:       result.Success,
		Message:       result.Message,
		TransactionID: result.TransID,
	})
}

// fetchAndScoreServer looks up a server from the most recent scan data,
// converts it, and scores it using the current config.
func (s *server) fetchAndScoreServer(serverID int) (*scanner.Server, float64, *scorer.Breakdown, error) {
	rec, err := s.store.GetByServerID(serverID)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to query scan data: %w", err)
	}
	if rec == nil {
		return nil, 0, nil, fmt.Errorf("server %d not found in recent scans", serverID)
	}

	srv := scanRecordToServer(rec)
	var bd scorer.Breakdown
	if rec.BreakdownJSON != "" && rec.BreakdownJSON != "{}" {
		json.Unmarshal([]byte(rec.BreakdownJSON), &bd) //nolint:errcheck
	}
	return &srv, rec.Score, &bd, nil
}
