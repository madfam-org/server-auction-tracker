package cpu

import "strings"

// PassMarkScores maps common Hetzner auction CPU models to PassMark multi-thread scores.
// Source: cpubenchmark.net, updated periodically.
var PassMarkScores = map[string]int{
	// AMD Ryzen 5000 series
	"AMD Ryzen 9 5950X": 39695,
	"AMD Ryzen 9 5900X": 34242,
	"AMD Ryzen 7 5800X": 28508,
	"AMD Ryzen 7 5700X": 26280,
	"AMD Ryzen 5 5600X": 22078,
	"AMD Ryzen 5 5600":  21195,
	// AMD Ryzen 3000 series
	"AMD Ryzen 9 3900X": 31902,
	"AMD Ryzen 9 3900":  31200,
	"AMD Ryzen 7 3700X": 22779,
	"AMD Ryzen 5 3600":  17823,
	"AMD Ryzen 5 3600X": 18264,
	// AMD Ryzen 7000 series
	"AMD Ryzen 9 7950X": 63379,
	"AMD Ryzen 9 7900X": 52824,
	"AMD Ryzen 7 7700X": 36074,
	"AMD Ryzen 5 7600X": 28483,
	// AMD EPYC
	"AMD EPYC 7443P": 42424,
	"AMD EPYC 7402P": 30920,
	"AMD EPYC 7302P": 24990,
	"AMD EPYC 7282":  20820,
	"AMD EPYC 7551P": 21900,
	"AMD EPYC 7501P": 20480,
	"AMD EPYC 7401P": 18820,
	"AMD EPYC 9254":  34520,
	"AMD EPYC 9354P": 61300,
	"AMD EPYC 9454P": 78200,
	// Intel Core i9
	"Intel Core i9-13900":  41495,
	"Intel Core i9-13900K": 41495,
	"Intel Core i9-12900":  35393,
	"Intel Core i9-12900K": 35393,
	"Intel Core i9-10900":  20026,
	"Intel Core i9-10900K": 21534,
	// Intel Core i7
	"Intel Core i7-13700":  38614,
	"Intel Core i7-13700K": 38614,
	"Intel Core i7-12700":  31136,
	"Intel Core i7-12700K": 31136,
	"Intel Core i7-10700":  16455,
	"Intel Core i7-10700K": 17085,
	"Intel Core i7-9700":   13822,
	"Intel Core i7-9700K":  14322,
	"Intel Core i7-8700":   13128,
	"Intel Core i7-8700K":  13614,
	"Intel Core i7-7700":   9578,
	"Intel Core i7-7700K":  10014,
	"Intel Core i7-6700":   8930,
	"Intel Core i7-6700K":  9392,
	// Intel Core i5
	"Intel Core i5-13500": 27805,
	"Intel Core i5-13400": 24282,
	"Intel Core i5-12500": 19564,
	"Intel Core i5-12400": 19472,
	"Intel Core i5-10600": 12614,
	"Intel Core i5-10400": 12263,
	// Intel Xeon E
	"Intel Xeon E-2388G": 21210,
	"Intel Xeon E-2378":  18120,
	"Intel Xeon E-2378G": 18400,
	"Intel Xeon E-2288G": 15180,
	"Intel Xeon E-2278G": 14200,
	"Intel Xeon E-2236":  12950,
	"Intel Xeon E-2176G": 13390,
	"Intel Xeon E-2136":  12900,
	"Intel Xeon E-2186G": 14900,
	"Intel Xeon E-2246G": 13200,
	"Intel Xeon E-2276G": 13750,
	"Intel Xeon E-2286G": 14900,
	"Intel Xeon E-2386G": 18560,
	"Intel Xeon E-2324G": 10710,
	"Intel Xeon E-2334":  12680,
	"Intel Xeon E-2336":  15160,
	"Intel Xeon E-2356G": 17240,
	// Intel Xeon W
	"Intel Xeon W-2145": 14360,
	"Intel Xeon W-2175": 18740,
	"Intel Xeon W-2195": 21480,
	"Intel Xeon W-2235": 12580,
	"Intel Xeon W-2255": 17260,
	"Intel Xeon W-2295": 24660,
	// Intel Xeon Scalable
	"Intel Xeon Gold 6226R":  21480,
	"Intel Xeon Gold 6230":   22230,
	"Intel Xeon Gold 6246":   20160,
	"Intel Xeon Gold 6248":   23460,
	"Intel Xeon Gold 6330":   38580,
	"Intel Xeon Gold 6342":   43260,
	"Intel Xeon Gold 6354":   41160,
	"Intel Xeon Silver 4210": 12990,
	"Intel Xeon Silver 4214": 14230,
	"Intel Xeon Silver 4310": 18960,
	"Intel Xeon Silver 4314": 26880,
	// Older Intel
	"Intel Core i7-4770": 7290,
	"Intel Core i7-3770": 7210,
}

// LookupBenchmark returns the PassMark score for a CPU model.
// It tries exact match first, then fuzzy substring matching.
func LookupBenchmark(model string) (int, bool) {
	// Exact match
	if score, ok := PassMarkScores[model]; ok {
		return score, true
	}

	// Fuzzy match: find best match by checking if the model contains a known key or vice versa
	modelUpper := strings.ToUpper(model)
	bestScore := 0
	bestLen := 0
	for key, score := range PassMarkScores {
		keyUpper := strings.ToUpper(key)
		if strings.Contains(modelUpper, keyUpper) || strings.Contains(keyUpper, modelUpper) {
			// Prefer longer (more specific) matches
			if len(key) > bestLen {
				bestScore = score
				bestLen = len(key)
			}
		}
	}
	if bestScore > 0 {
		return bestScore, true
	}

	return 0, false
}
