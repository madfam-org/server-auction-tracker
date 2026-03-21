package notify

import (
	"fmt"
	"strings"

	"github.com/madfam-org/server-auction-tracker/internal/store"
)

// FormatDigest builds a summary string from top deals for notification dispatch.
func FormatDigest(deals []store.ScanRecord, period string) string {
	if len(deals) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Deal Sniper %s Digest — Top %d deals\n\n", period, len(deals)))

	for i := range deals {
		b.WriteString(fmt.Sprintf(
			"%d. #%d %s — €%.2f (score %.1f) [%s]",
			i+1, deals[i].ServerID, deals[i].CPU, deals[i].Price, deals[i].Score, deals[i].Datacenter,
		))
		if deals[i].IsECC {
			b.WriteString(" [ECC]")
		}
		b.WriteString("\n")
	}

	return b.String()
}
