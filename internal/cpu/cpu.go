package cpu

import (
	"regexp"
	"strconv"
	"strings"
)

type Info struct {
	Model      string
	Brand      string // "AMD" or "Intel"
	Generation int
	Cores      int
	Threads    int
	BaseGHz    float64
}

var (
	ryzenRe     = regexp.MustCompile(`(?i)Ryzen\s+(\d)\s+(\d)(\d)\d\d`)
	intelRe     = regexp.MustCompile(`(?i)(?:Core\s+)?i(\d)-(\d+)(\d)\d\d`)
	epycRe      = regexp.MustCompile(`(?i)EPYC\s+(\d)(\d)\d\d`)
	xeonRe      = regexp.MustCompile(`(?i)Xeon\s+\w?-?(\d)(\d)\d\d`)
	ghzRe       = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*GHz`)
	coreCountRe = regexp.MustCompile(`(\d+)-Core`)
)

// Parse extracts CPU information from a model string.
// If cores/threads are provided (non-zero), they are used directly.
// Otherwise, cores are estimated from the model name.
func Parse(model string, cores, threads int, benchmarkScore int) Info {
	info := Info{
		Model: model,
	}

	info.BaseGHz = extractGHz(model)
	info.Brand, info.Generation = extractBrandGen(model)

	// Try to get cores from the model string (e.g., "6-Core")
	if cores == 0 {
		cores = extractCoreCount(model)
	}
	info.Cores = cores

	if threads == 0 && cores > 0 {
		threads = cores * 2 // SMT/HT default
	}
	info.Threads = threads

	return info
}

func extractCoreCount(model string) int {
	m := coreCountRe.FindStringSubmatch(model)
	if m != nil {
		c, err := strconv.Atoi(m[1])
		if err == nil {
			return c
		}
	}
	return 0
}

func extractGHz(model string) float64 {
	m := ghzRe.FindStringSubmatch(model)
	if m != nil {
		ghz, err := strconv.ParseFloat(m[1], 64)
		if err == nil {
			return ghz
		}
	}
	return 0
}

func extractBrandGen(model string) (string, int) {
	upper := strings.ToUpper(model)

	if strings.Contains(upper, "RYZEN") || strings.Contains(upper, "EPYC") {
		brand := "AMD"

		m := ryzenRe.FindStringSubmatch(model)
		if m != nil {
			gen, _ := strconv.Atoi(m[2])
			return brand, gen
		}

		m = epycRe.FindStringSubmatch(model)
		if m != nil {
			gen, _ := strconv.Atoi(m[1])
			return brand, gen
		}

		return brand, 0
	}

	if strings.Contains(upper, "INTEL") || strings.Contains(upper, "CORE") ||
		strings.Contains(upper, "XEON") || strings.Contains(upper, "I5") ||
		strings.Contains(upper, "I7") || strings.Contains(upper, "I9") {
		brand := "Intel"

		m := intelRe.FindStringSubmatch(model)
		if m != nil {
			genStr := m[2]
			if len(genStr) >= 2 {
				gen, _ := strconv.Atoi(genStr[:2])
				return brand, gen
			}
			gen, _ := strconv.Atoi(genStr)
			return brand, gen
		}

		m = xeonRe.FindStringSubmatch(model)
		if m != nil {
			gen, _ := strconv.Atoi(m[1])
			return brand, gen
		}

		return brand, 0
	}

	return "Unknown", 0
}

func GenerationScore(generation int) float64 {
	switch {
	case generation >= 13:
		return 1.0
	case generation >= 10:
		return 0.8
	case generation >= 7:
		return 0.6
	case generation >= 5:
		return 0.4
	case generation >= 3:
		return 0.3
	default:
		return 0.2
	}
}
