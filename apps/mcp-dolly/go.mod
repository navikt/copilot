module github.com/navikt/copilot/apps/mcp-dolly

go 1.26.0

require github.com/jackc/pgx/v5 v5.13.0

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

require github.com/navikt/copilot/lib/mcpkit v0.0.0

replace github.com/navikt/copilot/lib/mcpkit => ../../lib/mcpkit
