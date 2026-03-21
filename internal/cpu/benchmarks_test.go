package cpu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookupBenchmarkExact(t *testing.T) {
	score, ok := LookupBenchmark("AMD Ryzen 5 3600")
	assert.True(t, ok)
	assert.Equal(t, 17823, score)
}

func TestLookupBenchmarkFuzzy(t *testing.T) {
	// Model string from Hetzner includes extra text like "6-Core Processor"
	score, ok := LookupBenchmark("AMD Ryzen 5 3600 6-Core Processor")
	assert.True(t, ok)
	assert.Equal(t, 17823, score)
}

func TestLookupBenchmarkMiss(t *testing.T) {
	score, ok := LookupBenchmark("Some Unknown CPU 9999")
	assert.False(t, ok)
	assert.Equal(t, 0, score)
}

func TestLookupBenchmarkIntel(t *testing.T) {
	score, ok := LookupBenchmark("Intel Core i7-13700")
	assert.True(t, ok)
	assert.Equal(t, 38614, score)
}

func TestLookupBenchmarkEPYC(t *testing.T) {
	score, ok := LookupBenchmark("AMD EPYC 7443P 24-Core Processor")
	assert.True(t, ok)
	assert.Equal(t, 42424, score)
}
