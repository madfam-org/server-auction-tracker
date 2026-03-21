package order

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckEligibilityAllPass(t *testing.T) {
	client := NewRobotClient(&config.Order{Enabled: true, MinScore: 90, MaxPriceEUR: 80})
	server := scanner.Server{ID: 1001, Price: 39}
	check := client.CheckEligibility(&server, 92.0, &config.Order{Enabled: true, MinScore: 90, MaxPriceEUR: 80})

	assert.True(t, check.Eligible)
	assert.Empty(t, check.Reasons)
}

func TestCheckEligibilityDisabled(t *testing.T) {
	client := NewRobotClient(&config.Order{Enabled: false})
	server := scanner.Server{ID: 1001, Price: 39}
	check := client.CheckEligibility(&server, 95.0, &config.Order{Enabled: false, MinScore: 90, MaxPriceEUR: 80})

	assert.False(t, check.Eligible)
	assert.Contains(t, check.Reasons[0], "disabled")
}

func TestCheckEligibilityScoreTooLow(t *testing.T) {
	client := NewRobotClient(&config.Order{Enabled: true, MinScore: 90, MaxPriceEUR: 80})
	server := scanner.Server{ID: 1001, Price: 39}
	check := client.CheckEligibility(&server, 85.0, &config.Order{Enabled: true, MinScore: 90, MaxPriceEUR: 80})

	assert.False(t, check.Eligible)
	assert.Contains(t, check.Reasons[0], "score 85.0 below minimum 90.0")
}

func TestCheckEligibilityPriceTooHigh(t *testing.T) {
	client := NewRobotClient(&config.Order{Enabled: true, MinScore: 90, MaxPriceEUR: 80})
	server := scanner.Server{ID: 1001, Price: 95}
	check := client.CheckEligibility(&server, 92.0, &config.Order{Enabled: true, MinScore: 90, MaxPriceEUR: 80})

	assert.False(t, check.Eligible)
	assert.Contains(t, check.Reasons[0], "price")
}

func TestCheckEligibilityMultipleFailures(t *testing.T) {
	client := NewRobotClient(&config.Order{})
	server := scanner.Server{ID: 1001, Price: 95}
	check := client.CheckEligibility(&server, 50.0, &config.Order{Enabled: false, MinScore: 90, MaxPriceEUR: 80})

	assert.False(t, check.Eligible)
	assert.Len(t, check.Reasons, 3) // disabled + score + price
}

func TestRobotOrderSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/order/server_market/transaction")

		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "testuser", user)
		assert.Equal(t, "testpass", pass)

		resp := robotOrderResponse{}
		resp.Transaction.ID = "TX-12345"
		resp.Transaction.Status = "completed"
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewRobotClient(&config.Order{
		RobotURL:      srv.URL,
		RobotUser:     "testuser",
		RobotPassword: "testpass",
	})

	result, err := client.Order(context.Background(), 1001)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "TX-12345", result.TransID)
}

func TestRobotOrderHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "server not found"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	client := NewRobotClient(&config.Order{RobotURL: srv.URL})
	result, err := client.Order(context.Background(), 9999)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "HTTP 404")
}
