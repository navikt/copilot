// Command copilot-metrics copies GitHub Copilot usage, billing and Mimir data to BigQuery.
package main

import "github.com/navikt/copilot/apps/copilot-metrics/internal/jobs"

func main() {
	jobs.Run()
}
