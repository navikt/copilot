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

var adoptionViews = []viewDefinition{
	{name: "v_adoption_summary", filename: "views/v_adoption_summary.sql"},
	{name: "v_team_adoption", filename: "views/v_team_adoption.sql"},
	{name: "v_customization_details", filename: "views/v_customization_details.sql"},
	{name: "v_language_adoption", filename: "views/v_language_adoption.sql"},
	{name: "v_staleness_summary", filename: "views/v_staleness_summary.sql"},
}

func (c *BigQueryClient) EnsureViewsExist(ctx context.Context) error {
	for _, v := range adoptionViews {
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

	// Replace view reference: `%s.%s.viewname`
	viewRef := fmt.Sprintf("`%s.%s.%s`", c.projectID, c.dataset, v.name)
	sql = strings.Replace(sql, fmt.Sprintf("`%%s.%%s.%s`", v.name), viewRef, 1)

	// Replace all source table references: `%s.%s.%s`
	tableRef := fmt.Sprintf("`%s.%s.%s`", c.projectID, c.dataset, c.table)
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
