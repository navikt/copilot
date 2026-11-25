package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

const mimirDailyName = "mimir_daily"

var MimirDailySchema = bigquery.Schema{
	{Name: "day", Type: bigquery.DateFieldType},
	{Name: "family", Type: bigquery.StringFieldType},
	{Name: "metric", Type: bigquery.StringFieldType},
	{Name: "job", Type: bigquery.StringFieldType},
	{Name: "model", Type: bigquery.StringFieldType},
	{Name: "tool", Type: bigquery.StringFieldType},
	{Name: "token_type", Type: bigquery.StringFieldType},
	{Name: "operation", Type: bigquery.StringFieldType},
	{Name: "error_type", Type: bigquery.StringFieldType},
	{Name: "success", Type: bigquery.BooleanFieldType},
	{Name: "edit_outcome", Type: bigquery.StringFieldType},
	{Name: "edit_source", Type: bigquery.StringFieldType},
	{Name: "time_delay_ms", Type: bigquery.IntegerFieldType},
	{Name: "client", Type: bigquery.StringFieldType},
	{Name: "version", Type: bigquery.StringFieldType},
	{Name: "skill", Type: bigquery.StringFieldType},
	{Name: "source", Type: bigquery.StringFieldType},
	{Name: "trigger", Type: bigquery.StringFieldType},
	{Name: "outcome", Type: bigquery.StringFieldType},
	{Name: "hook_type", Type: bigquery.StringFieldType},
	{Name: "agent", Type: bigquery.StringFieldType},
	{Name: "shutdown_type", Type: bigquery.StringFieldType},
	{Name: "devices", Type: bigquery.IntegerFieldType},
	{Name: "counter_reliable", Type: bigquery.BooleanFieldType},
	{Name: "increase", Type: bigquery.FloatFieldType},
	{Name: "hist_count", Type: bigquery.FloatFieldType},
	{Name: "hist_sum", Type: bigquery.FloatFieldType},
	{Name: "p50", Type: bigquery.FloatFieldType},
	{Name: "p95", Type: bigquery.FloatFieldType},
	{Name: "cache_read_share", Type: bigquery.FloatFieldType},
	{Name: "input_output_ratio", Type: bigquery.FloatFieldType},
	// Appended, nullable: EnsureMimirDailyTableExists adds them to an existing table.
	{Name: "sandbox", Type: bigquery.StringFieldType},
	{Name: "cplt_version", Type: bigquery.StringFieldType},
	{Name: "os", Type: bigquery.StringFieldType},
}

func (c *BigQueryClient) EnsureMimirDailyTableExists(ctx context.Context) error {
	t := c.Client.Dataset(c.dataset).Table(mimirDailyName)
	if meta, err := t.Metadata(ctx); err == nil {
		if len(meta.Schema) < len(MimirDailySchema) {
			_, err = t.Update(ctx, bigquery.TableMetadataToUpdate{Schema: MimirDailySchema}, meta.ETag)
		}
		return err
	}
	return t.Create(ctx, &bigquery.TableMetadata{Schema: MimirDailySchema, Description: "Daily Mimir aggregates (nav-pilot / Copilot telemetry). Device-labelled families have k>=5 devices; copilot_chat and nav_pilot_session rows carry no device count."})
}

// ReplaceMimirDay loads rows into a scratch table, then swaps the day in one
// transaction, so a failed load never leaves the day empty.
func (c *BigQueryClient) ReplaceMimirDay(ctx context.Context, day time.Time, rows []types.MimirRow) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	scratch := c.Client.Dataset(c.dataset).Table(mimirDailyName + "_load_" + day.Format("20060102"))
	if err := scratch.Create(ctx, &bigquery.TableMetadata{Schema: MimirDailySchema, ExpirationTime: time.Now().Add(6 * time.Hour)}); err != nil && !strings.Contains(err.Error(), "Already Exists") {
		return err
	}
	src := bigquery.NewReaderSource(&buf)
	src.SourceFormat = bigquery.JSON
	src.Schema = MimirDailySchema
	l := scratch.LoaderFrom(src)
	l.WriteDisposition = bigquery.WriteTruncate
	if err := waitJob(ctx, l.Run); err != nil {
		return fmt.Errorf("load scratch table: %w", err)
	}
	cols := make([]string, len(MimirDailySchema))
	for i, f := range MimirDailySchema {
		cols[i] = f.Name
	}
	colList := strings.Join(cols, ", ")
	target := "`" + c.projectID + "." + c.dataset + "." + mimirDailyName + "`"
	q := c.Client.Query("BEGIN TRANSACTION;\n" +
		"DELETE FROM " + target + " WHERE day = @day;\n" +
		"INSERT INTO " + target + " (" + colList + ") SELECT " + colList + " FROM `" + c.projectID + "." + c.dataset + "." + scratch.TableID + "`;\n" +
		"COMMIT TRANSACTION;")
	q.Parameters = []bigquery.QueryParameter{{Name: "day", Value: civil.DateOf(day)}}
	if err := waitJob(ctx, q.Run); err != nil {
		return fmt.Errorf("swap day: %w", err)
	}
	if err := scratch.Delete(ctx); err != nil {
		slog.Warn("Could not drop scratch table; it expires in 6 h", "table", scratch.TableID, "error", err)
	}
	return nil
}

func waitJob(ctx context.Context, run func(context.Context) (*bigquery.Job, error)) error {
	job, err := run(ctx)
	if err != nil {
		return err
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return err
	}
	return status.Err()
}
