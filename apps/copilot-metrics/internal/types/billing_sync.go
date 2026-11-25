package types

import (
	"time"
)

var BillingSyncStart = time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)

// billingSKUs are stored per user and per enterprise; the residual compares
// the same set on both sides. Premium-request SKUs cover months before June
// 2026 and keep their GitHub names, so consumers can tell the two apart.
var BillingSKUs = []string{"Copilot AI Credits", "Copilot Cloud Agent", "Copilot Premium Request", "Coding Agent Premium Request"}
