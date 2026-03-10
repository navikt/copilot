package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"strings"
)

//go:embed views/*.sql
var viewsFS embed.FS

type viewDefinition struct {
	name     string
	filename string
}

var views = []viewDefinition{
	{name: "v_daily_summary", filename: "views/v_daily_summary.sql"},
	{name: "v_language_stats", filename: "views/v_language_stats.sql"},
	{name: "v_editor_stats", filename: "views/v_editor_stats.sql"},
	{name: "v_model_stats", filename: "views/v_model_stats.sql"},
}

func (c *BigQueryClient) EnsureViewsExist(ctx context.Context) error {
	for _, v := range views {
		if err := c.createOrReplaceView(ctx, v); err != nil {
			return fmt.Errorf("failed to create view %s: %w", v.name, err)
		}
	}
	return nil
}

func (c *BigQueryClient) createOrReplaceView(ctx context.Context, v viewDefinition) error {
	template, err := viewsFS.ReadFile(v.filename)
	if err != nil {
		return fmt.Errorf("failed to read view template %s: %w", v.filename, err)
	}

	sql := string(template)

	// Replace table reference placeholders: %s.%s.%s → projectID.dataset.table
	tableRef := fmt.Sprintf("`%s.%s.%s`", c.projectID, c.dataset, c.table)
	// Replace view reference placeholders: %s.%s.viewname → projectID.dataset.viewname
	viewRef := fmt.Sprintf("`%s.%s.%s`", c.projectID, c.dataset, v.name)

	// The SQL templates use %s.%s.%s for the source table and %s.%s.viewname for the view
	// First occurrence is the view name in CREATE OR REPLACE VIEW, rest are source table references
	parts := strings.SplitN(sql, fmt.Sprintf("`%%s.%%s.%s`", v.name), 2)
	if len(parts) == 2 {
		sql = parts[0] + viewRef + parts[1]
	}

	// Replace remaining %s.%s.%s patterns for source table
	sql = strings.ReplaceAll(sql, "`%s.%s.%s`", tableRef)

	slog.Info("Creating/updating view", "view", v.name)

	query := c.client.Query(sql)
	job, err := query.Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to run view creation: %w", err)
	}

	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("view creation job failed: %w", err)
	}
	if status.Err() != nil {
		return fmt.Errorf("view creation query failed: %w", status.Err())
	}

	slog.Info("View ready", "view", v.name)
	return nil
}
