package cpu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRyzen(t *testing.T) {
	info := Parse("AMD Ryzen 5 3600 6-Core Processor", 6, 12, 0)
	assert.Equal(t, "AMD", info.Brand)
	assert.Equal(t, 3, info.Generation)
	assert.Equal(t, 6, info.Cores)
	assert.Equal(t, 12, info.Threads)
}

func TestParseIntel(t *testing.T) {
	info := Parse("Intel Core i5-13500", 14, 20, 0)
	assert.Equal(t, "Intel", info.Brand)
	assert.Equal(t, 13, info.Generation)
	assert.Equal(t, 14, info.Cores)
	assert.Equal(t, 20, info.Threads)
}

func TestParseGHz(t *testing.T) {
	info := Parse("AMD Ryzen 5 3600 3.6GHz", 6, 12, 0)
	assert.InDelta(t, 3.6, info.BaseGHz, 0.01)
}

func TestParseUnknown(t *testing.T) {
	info := Parse("Some Custom CPU", 4, 8, 0)
	assert.Equal(t, "Unknown", info.Brand)
	assert.Equal(t, 0, info.Generation)
}

func TestParseEPYC(t *testing.T) {
	info := Parse("AMD EPYC 7402P", 24, 48, 0)
	assert.Equal(t, "AMD", info.Brand)
	assert.Equal(t, 7, info.Generation)
	assert.Equal(t, 24, info.Cores)
	assert.Equal(t, 48, info.Threads)
}

func TestParseXeonE(t *testing.T) {
	info := Parse("Intel Xeon E-2136", 6, 12, 0)
	assert.Equal(t, "Intel", info.Brand)
	assert.Equal(t, 6, info.Cores)
}

func TestParseEmptyString(t *testing.T) {
	info := Parse("", 0, 0, 0)
	assert.Equal(t, "Unknown", info.Brand)
	assert.Equal(t, 0, info.Generation)
	assert.Equal(t, 0, info.Cores)
}

func TestParseSuffixedModel(t *testing.T) {
	info := Parse("AMD Ryzen 9 5950X", 0, 0, 0)
	assert.Equal(t, "AMD", info.Brand)
	assert.Equal(t, 5, info.Generation)
}

func TestParseCoreCountFromString(t *testing.T) {
	info := Parse("AMD Ryzen 5 3600 6-Core Processor", 0, 0, 0)
	assert.Equal(t, 6, info.Cores)
	assert.Equal(t, 12, info.Threads)
}

func TestGenerationScore(t *testing.T) {
	tests := []struct {
		gen      int
		expected float64
	}{
		{13, 1.0},
		{14, 1.0},
		{10, 0.8},
		{12, 0.8},
		{7, 0.6},
		{9, 0.6},
		{5, 0.4},
		{3, 0.3},
		{1, 0.2},
		{0, 0.2},
	}
	for _, tt := range tests {
		assert.InDelta(t, tt.expected, GenerationScore(tt.gen), 0.001, "gen=%d", tt.gen)
	}
}
