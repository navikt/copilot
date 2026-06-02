package main

import (
	"testing"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

func field(name string, t bigquery.FieldType) *bigquery.FieldSchema {
	return &bigquery.FieldSchema{Name: name, Type: t}
}

// TestDecodeBQRow_NullsBecomeZero is the core resilience guarantee: a NULL value
// from BigQuery must leave the struct field at its zero value rather than failing
// the scan. This reproduces the production crash
// "NULL cannot be assigned to field AdoptionRate of type float64".
func TestDecodeBQRow_NullsBecomeZero(t *testing.T) {
	schema := bigquery.Schema{
		field("adoption_rate", bigquery.FloatFieldType),
		field("adoption_rate_active_only", bigquery.FloatFieldType),
		field("total_repos", bigquery.IntegerFieldType),
		field("language", bigquery.StringFieldType),
	}
	row := []bigquery.Value{nil, nil, nil, nil}

	var got LanguageAdoption
	if err := decodeBQRow(schema, row, &got); err != nil {
		t.Fatalf("decodeBQRow returned error on NULLs: %v", err)
	}
	if got.AdoptionRate != 0 || got.AdoptionRateActiveOnly != 0 {
		t.Errorf("expected zero rates, got %+v", got)
	}
	if got.TotalRepos != 0 {
		t.Errorf("expected zero total_repos, got %d", got.TotalRepos)
	}
	if got.Language != "" {
		t.Errorf("expected empty language, got %q", got.Language)
	}
}

// TestDecodeBQRow_FloatIntoInt reproduces the production crash
// "schema field gross_requests of type FLOAT is not assignable to struct field
// gross_requests of type int64": SUM() over a FLOAT column yields FLOAT, which
// must be coerced into the int64 field.
func TestDecodeBQRow_FloatIntoInt(t *testing.T) {
	schema := bigquery.Schema{
		field("gross_requests", bigquery.FloatFieldType),
		field("net_requests", bigquery.FloatFieldType),
		field("gross_amount", bigquery.FloatFieldType),
		field("month", bigquery.StringFieldType),
	}
	row := []bigquery.Value{float64(1234), float64(1000), 42.5, "2026-06"}

	var got MonthlyBillingUsage
	if err := decodeBQRow(schema, row, &got); err != nil {
		t.Fatalf("decodeBQRow returned error: %v", err)
	}
	if got.GrossRequests != 1234 {
		t.Errorf("GrossRequests = %d, want 1234", got.GrossRequests)
	}
	if got.NetRequests != 1000 {
		t.Errorf("NetRequests = %d, want 1000", got.NetRequests)
	}
	if got.GrossAmount != 42.5 {
		t.Errorf("GrossAmount = %v, want 42.5", got.GrossAmount)
	}
	if got.Month != "2026-06" {
		t.Errorf("Month = %q, want 2026-06", got.Month)
	}
}

// TestDecodeBQRow_IntIntoFloat covers the reverse coercion: an INTEGER value
// landing in a float64 field.
func TestDecodeBQRow_IntIntoFloat(t *testing.T) {
	schema := bigquery.Schema{field("avg_generations", bigquery.IntegerFieldType)}
	row := []bigquery.Value{int64(7)}

	var got AdoptionCohortDay
	if err := decodeBQRow(schema, row, &got); err != nil {
		t.Fatalf("decodeBQRow returned error: %v", err)
	}
	if got.AvgGenerations != 7 {
		t.Errorf("AvgGenerations = %v, want 7", got.AvgGenerations)
	}
}

func TestDecodeBQRow_DateAndScalars(t *testing.T) {
	schema := bigquery.Schema{
		field("day", bigquery.DateFieldType),
		field("phase", bigquery.IntegerFieldType),
		field("phase_version", bigquery.StringFieldType),
		field("user_count", bigquery.IntegerFieldType),
	}
	date := civil.Date{Year: 2026, Month: 6, Day: 2}
	row := []bigquery.Value{date, int64(3), "v2", int64(50)}

	var got AdoptionCohortDay
	if err := decodeBQRow(schema, row, &got); err != nil {
		t.Fatalf("decodeBQRow returned error: %v", err)
	}
	if got.Day != date {
		t.Errorf("Day = %v, want %v", got.Day, date)
	}
	if got.Phase != 3 || got.PhaseVersion != "v2" || got.UserCount != 50 {
		t.Errorf("unexpected scalars: %+v", got)
	}
}

// TestDecodeBQRow_RepeatedRecord covers ARRAY<STRUCT> columns (top_models) and a
// NULL array, which must produce an empty slice rather than an error.
func TestDecodeBQRow_RepeatedRecord(t *testing.T) {
	modelsField := &bigquery.FieldSchema{
		Name:     "top_models",
		Type:     bigquery.RecordFieldType,
		Repeated: true,
		Schema: bigquery.Schema{
			field("model", bigquery.StringFieldType),
			field("interactions", bigquery.IntegerFieldType),
		},
	}
	schema := bigquery.Schema{field("team_slug", bigquery.StringFieldType), modelsField}
	row := []bigquery.Value{
		"team-a",
		[]bigquery.Value{
			[]bigquery.Value{"gpt", int64(10)},
			[]bigquery.Value{"claude", float64(5)}, // FLOAT coerced into int64 within nested record
		},
	}

	var got TeamUsageSummary
	if err := decodeBQRow(schema, row, &got); err != nil {
		t.Fatalf("decodeBQRow returned error: %v", err)
	}
	if got.TeamSlug != "team-a" {
		t.Errorf("TeamSlug = %q", got.TeamSlug)
	}
	if len(got.TopModels) != 2 {
		t.Fatalf("expected 2 models, got %d", len(got.TopModels))
	}
	if got.TopModels[0].Model != "gpt" || got.TopModels[0].Interactions != 10 {
		t.Errorf("model[0] = %+v", got.TopModels[0])
	}
	if got.TopModels[1].Model != "claude" || got.TopModels[1].Interactions != 5 {
		t.Errorf("model[1] = %+v", got.TopModels[1])
	}
}

func TestDecodeBQRow_RepeatedString(t *testing.T) {
	teamsField := &bigquery.FieldSchema{Name: "teams", Type: bigquery.StringFieldType, Repeated: true}
	schema := bigquery.Schema{field("user_login", bigquery.StringFieldType), teamsField}
	row := []bigquery.Value{"octocat", []bigquery.Value{"team-a", "team-b"}}

	var got UserMetricsSummary
	if err := decodeBQRow(schema, row, &got); err != nil {
		t.Fatalf("decodeBQRow returned error: %v", err)
	}
	if len(got.Teams) != 2 || got.Teams[0] != "team-a" || got.Teams[1] != "team-b" {
		t.Errorf("Teams = %v", got.Teams)
	}
}

func TestDecodeBQRow_NullRepeatedIsEmpty(t *testing.T) {
	teamsField := &bigquery.FieldSchema{Name: "teams", Type: bigquery.StringFieldType, Repeated: true}
	schema := bigquery.Schema{teamsField}
	row := []bigquery.Value{nil}

	var got UserMetricsSummary
	if err := decodeBQRow(schema, row, &got); err != nil {
		t.Fatalf("decodeBQRow returned error: %v", err)
	}
	if got.Teams != nil {
		t.Errorf("expected nil/empty Teams, got %v", got.Teams)
	}
}

// TestDecodeBQRow_SchemaDrift verifies the decoder tolerates columns that have no
// matching struct field (and vice versa) instead of crashing.
func TestDecodeBQRow_SchemaDrift(t *testing.T) {
	schema := bigquery.Schema{
		field("language", bigquery.StringFieldType),
		field("brand_new_column", bigquery.IntegerFieldType), // no struct field
	}
	row := []bigquery.Value{"go", int64(99)}

	var got LanguageAdoption
	if err := decodeBQRow(schema, row, &got); err != nil {
		t.Fatalf("decodeBQRow returned error on unknown column: %v", err)
	}
	if got.Language != "go" {
		t.Errorf("Language = %q, want go", got.Language)
	}
}

func TestDecodeBQRow_TypeMismatchErrors(t *testing.T) {
	// A genuinely incompatible value (string into a float field) should surface an
	// error rather than silently corrupting data.
	schema := bigquery.Schema{field("adoption_rate", bigquery.FloatFieldType)}
	row := []bigquery.Value{"not-a-number"}

	var got LanguageAdoption
	if err := decodeBQRow(schema, row, &got); err == nil {
		t.Fatal("expected an error for string-into-float, got nil")
	}
}

func TestDecodeBQRow_RejectsNonPointer(t *testing.T) {
	schema := bigquery.Schema{field("language", bigquery.StringFieldType)}
	row := []bigquery.Value{"go"}

	if err := decodeBQRow(schema, row, LanguageAdoption{}); err == nil {
		t.Fatal("expected an error when target is not a pointer")
	}
}
