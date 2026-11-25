package survey

import (
	"encoding/base64"
	"log/slog"
	"os"
	"strings"
	"time"
)

// surveyKeys reads each survey's secret key from SURVEY_KEY_<ID> (the id
// upper-cased, - as _), base64 of at least 32 bytes. A survey with a missing
// or short key is not listed and takes no answers; an active one is
// logged as an error.
//
// A key still present after its survey closed is logged as a warning: it
// should have been deleted (see surveys/README.md, key lifecycle).
func surveyKeys(surveys []survey, now time.Time) map[string][]byte {
	keys := map[string][]byte{}
	for _, s := range surveys {
		name := "SURVEY_KEY_" + strings.ToUpper(strings.ReplaceAll(s.ID, "-", "_"))
		if !now.Before(s.closesOn()) {
			if os.Getenv(name) != "" {
				slog.Warn("survey key still present after the survey closed: delete it from the copilot-survey secret", "key", name)
			}
			continue
		}
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv(name)))
		if err == nil && len(key) >= 32 {
			keys[s.ID] = key
		} else if s.activeOn(now) {
			slog.Error("active survey has no valid key: it is not listed and takes no answers", "key", name)
		}
	}
	return keys
}
