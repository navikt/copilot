package main

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"cloud.google.com/go/civil"
)

const userBillingPause = 2 * time.Second

type userBillingFetcher interface {
	FetchUserAICreditUsage(context.Context, string, time.Time) (*BillingUsageResponse, error)
}

type userBillingStore interface {
	UserBillingRunComplete(context.Context, time.Time, string) (bool, error)
	GetBillingUsers(context.Context, time.Time, string) (map[string]string, error)
	UserBillingDone(context.Context, time.Time, string) (map[string]bool, error)
	ReplaceUserBilling(context.Context, []UserBillingRow) error
	CompleteUserBilling(context.Context, time.Time, string, int) error
}

// ingestUserBillingMonth is separate from --run-once. A full month needs
// hundreds of requests and should not spend the shared PAT budget nightly.
func ingestUserBillingMonth(ctx context.Context, client userBillingFetcher, store userBillingStore, cfg *Config, month time.Time) error {
	complete, err := store.UserBillingRunComplete(ctx, month, cfg.EnterpriseSlug)
	if err != nil {
		return fmt.Errorf("check billing month: %w", err)
	}
	if complete {
		slog.Info("User billing month already complete", "month", month.Format("2006-01"))
		return nil
	}
	users, err := store.GetBillingUsers(ctx, month, cfg.EnterpriseSlug)
	if err != nil {
		return fmt.Errorf("list billing users: %w", err)
	}
	done, err := store.UserBillingDone(ctx, month, cfg.EnterpriseSlug)
	if err != nil {
		return fmt.Errorf("list completed billing users: %w", err)
	}
	ids := make([]string, 0, len(users))
	for id := range users {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for index, id := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if done[id] {
			continue
		}
		response, err := client.FetchUserAICreditUsage(ctx, users[id], month)
		if err != nil {
			return fmt.Errorf("billing user %d/%d: %w", index+1, len(ids), err)
		}
		if err := validateUserBilling(response, users[id], cfg.EnterpriseSlug, month); err != nil {
			return fmt.Errorf("billing user %d/%d: %w", index+1, len(ids), err)
		}
		bySKU := map[string]UserBillingRow{}
		for _, item := range response.UsageItems {
			if item.Product != "Copilot" || (item.SKU != "Copilot AI Credits" && item.SKU != "Copilot Cloud Agent") {
				continue
			}
			row := bySKU[item.SKU]
			row.Month = civil.DateOf(month)
			row.ScopeID = cfg.EnterpriseSlug
			row.UserID = id
			row.GitHubLogin = users[id]
			row.SKU = item.SKU
			row.LoadedAt = time.Now().UTC()
			row.GrossAmount += item.GrossAmount
			row.NetAmount += item.NetAmount
			bySKU[item.SKU] = row
		}
		rows := []UserBillingRow{{
			Month: civil.DateOf(month), ScopeID: cfg.EnterpriseSlug, UserID: id,
			GitHubLogin: users[id], SKU: "done", LoadedAt: time.Now().UTC(),
		}}
		for _, row := range bySKU {
			rows = append(rows, row)
		}
		if err := store.ReplaceUserBilling(ctx, rows); err != nil {
			return fmt.Errorf("replace billing user %d/%d: %w", index+1, len(ids), err)
		}
		if (index+1)%100 == 0 {
			slog.Info("User billing progress", "processed", index+1, "total", len(ids))
		}
		if index == len(ids)-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(userBillingPause):
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("no billing users for %s", month.Format("2006-01"))
	}
	return store.CompleteUserBilling(ctx, month, cfg.EnterpriseSlug, len(ids))
}
