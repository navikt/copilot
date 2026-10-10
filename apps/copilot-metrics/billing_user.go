package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"
)

const userBillingPause = 2 * time.Second

type userBillingFetcher interface {
	FetchUserAICreditUsage(context.Context, string, time.Time) (*BillingUsageResponse, error)
	AccountDeleted(context.Context, string) (bool, error)
}

// deletedAccount reports whether a 404 for a login means the account is gone.
// A renamed account also 404s on its old login but still resolves by id, so
// that case and every other error are returned unchanged.
func deletedAccount(ctx context.Context, client userBillingFetcher, id string, err error) (bool, error) {
	var status *billingHTTPError
	if !errors.As(err, &status) || status.Status != http.StatusNotFound {
		return false, err
	}
	gone, lookupErr := client.AccountDeleted(ctx, id)
	if lookupErr != nil {
		return false, errors.Join(err, lookupErr)
	}
	if !gone {
		return false, err
	}
	slog.Warn("User billing skipped deleted account", "user_id", id)
	return true, nil
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
	if !month.Before(billingSyncStart) {
		return fmt.Errorf("months from October 2026 require validated automatic billing sync")
	}
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
	skipped := 0
	for index, id := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if done[id] {
			continue
		}
		response, err := client.FetchUserAICreditUsage(ctx, users[id], month)
		if gone, goneErr := deletedAccount(ctx, client, id, err); goneErr != nil {
			return fmt.Errorf("billing user %d/%d: %w", index+1, len(ids), goneErr)
		} else if gone {
			skipped++
			response = &BillingUsageResponse{} // done marker only
		} else if err := validateUserBilling(response, users[id], cfg.EnterpriseSlug, month); err != nil {
			return fmt.Errorf("billing user %d/%d: %w", index+1, len(ids), err)
		}
		rows := billingRows(response, month, cfg.EnterpriseSlug, id, users[id])
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
	if err := store.CompleteUserBilling(ctx, month, cfg.EnterpriseSlug, len(ids)); err != nil {
		return err
	}
	slog.Info("User billing month complete", "month", month.Format("2006-01"), "users", len(ids), "skipped_deleted", skipped)
	return nil
}
