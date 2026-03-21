package notify

import (
	"testing"
	"time"

	"github.com/madfam-org/server-auction-tracker/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestFormatDigest(t *testing.T) {
	deals := []store.ScanRecord{
		{
			ServerID: 1001, CPU: "AMD Ryzen 9 5950X", Price: 72.50,
			Score: 95.0, Datacenter: "HEL1-DC7", IsECC: true,
			ScannedAt: time.Now(),
		},
		{
			ServerID: 1002, CPU: "Intel Core i7-13700", Price: 55.00,
			Score: 88.0, Datacenter: "FSN1-DC14",
			ScannedAt: time.Now(),
		},
	}

	text := FormatDigest(deals, "daily")
	assert.Contains(t, text, "daily Digest")
	assert.Contains(t, text, "Top 2 deals")
	assert.Contains(t, text, "#1001")
	assert.Contains(t, text, "AMD Ryzen 9 5950X")
	assert.Contains(t, text, "€72.50")
	assert.Contains(t, text, "[ECC]")
	assert.Contains(t, text, "#1002")
	assert.Contains(t, text, "Intel Core i7-13700")
}

func TestFormatDigestEmpty(t *testing.T) {
	text := FormatDigest(nil, "weekly")
	assert.Empty(t, text)
}

func TestFormatDigestWeekly(t *testing.T) {
	deals := []store.ScanRecord{
		{ServerID: 1, CPU: "test", Price: 40.0, Score: 80.0, Datacenter: "HEL1"},
	}
	text := FormatDigest(deals, "weekly")
	assert.Contains(t, text, "weekly Digest")
}
