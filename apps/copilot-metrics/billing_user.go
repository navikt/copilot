package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
)

const userBillingPause = 2 * time.Second

type userBillingFetcher interface {
	FetchUserAICreditUsage(context.Context, string, time.Time) (*BillingUsageResponse, error)
	AccountLogin(context.Context, string) (string, error)
	FetchEnterpriseAICreditUsage(context.Context, time.Time) (*BillingUsageResponse, error)
}

// billingUsers lists the month's users without bot accounts. GitHub answers a
// bot login with another user name and no usage, which fails validation.
func billingUsers(ctx context.Context, store userBillingStore, month time.Time, scope string) (map[string]string, error) {
	users, err := store.GetBillingUsers(ctx, month, scope)
	if err != nil {
		return nil, err
	}
	bots := 0
	for id, login := range users {
		if strings.HasSuffix(strings.ToLower(login), "[bot]") {
			delete(users, id)
			bots++
		}
	}
	if bots > 0 {
		slog.Info("User billing excluded bot accounts", "month", month.Format("2006-01"), "bots", bots)
	}
	return users, nil
}

// accountAfterNotFound looks an account up by id after a 404 on its login.
// handled is true for a deleted account (current "") and for a renamed one
// (current is the new login). Any other case returns err unchanged.
func accountAfterNotFound(ctx context.Context, client userBillingFetcher, id, login string, err error) (current string, handled bool, _ error) {
	var status *billingHTTPError
	if !errors.As(err, &status) || status.Status != http.StatusNotFound {
		return "", false, err
	}
	current, lookupErr := client.AccountLogin(ctx, id)
	if lookupErr != nil {
		return "", false, errors.Join(lookupErr, err) // lookup first: errors.As sees its status before the 404
	}
	if current == "" {
		slog.Warn("User billing skipped deleted account", "user_id", id)
		return "", true, nil
	}
	if strings.EqualFold(current, login) {
		return "", false, err
	}
	slog.Info("User billing account renamed, using current login", "user_id", id)
	return strings.ToLower(current), true, nil
}

type userBillingStore interface {
	UserBillingRunComplete(context.Context, time.Time, string) (bool, error)
	GetBillingUsers(context.Context, time.Time, string) (map[string]string, error)
	UserBillingDone(context.Context, time.Time, string) (map[string]bool, error)
	ReplaceUserBilling(context.Context, []UserBillingRow) error
	UserBillingNet(context.Context, time.Time, string) (float64, error)
	CompleteUserBilling(ctx context.Context, month time.Time, scope string, users int, enterpriseNet, userNet float64) error
}

// publishBillingMonth stores the enterprise total, the residual against the
// per-user rows and the completion marker. Both writers end here.
func publishBillingMonth(ctx context.Context, client userBillingFetcher, store userBillingStore, cfg *Config, month time.Time, users int) error {
	// Returned unwrapped: a bare errBillingBudget is the sync's clean checkpoint.
	enterprise, err := client.FetchEnterpriseAICreditUsage(ctx, month)
	if err != nil {
		return err
	}
	if err := validateUserBilling(enterprise, "", cfg.EnterpriseSlug, month); err != nil {
		return err
	}
	rows := billingRows(enterprise, month, cfg.EnterpriseSlug, "", "")
	enterpriseNet := 0.0
	for _, row := range rows {
		enterpriseNet += row.NetAmount
	}
	if err := store.ReplaceUserBilling(ctx, rows); err != nil {
		return err
	}
	userNet, err := store.UserBillingNet(ctx, month, cfg.EnterpriseSlug)
	if err != nil {
		return err
	}
	// The residual can be nonzero: users with charges but no metrics rows are
	// not in the census. It is recorded per run and never blocks.
	if err := store.CompleteUserBilling(ctx, month, cfg.EnterpriseSlug, users, enterpriseNet, userNet); err != nil {
		return err
	}
	slog.Info("User billing month published", "month", month.Format("2006-01"), "users", users,
		"enterprise_net", enterpriseNet, "user_net", userNet, "residual_net", enterpriseNet-userNet)
	return nil
}

// republishBillingMonth reruns only the publish step for a month whose users
// are already complete, to backfill its enterprise row and residual.
func republishBillingMonth(ctx context.Context, client userBillingFetcher, store userBillingStore, cfg *Config, month time.Time) error {
	complete, err := store.UserBillingRunComplete(ctx, month, cfg.EnterpriseSlug)
	if err != nil {
		return err
	}
	if !complete {
		return fmt.Errorf("%s has no completion marker; collect its users first", month.Format("2006-01"))
	}
	done, err := store.UserBillingDone(ctx, month, cfg.EnterpriseSlug)
	if err != nil {
		return err
	}
	delete(done, "")
	return publishBillingMonth(ctx, client, store, cfg, month, len(done))
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
	users, err := billingUsers(ctx, store, month, cfg.EnterpriseSlug)
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
		login := users[id]
		response, err := client.FetchUserAICreditUsage(ctx, login, month)
		current, handled, err := accountAfterNotFound(ctx, client, id, login, err)
		if err != nil {
			return fmt.Errorf("billing user %d/%d: %w", index+1, len(ids), err)
		}
		if handled && current == "" {
			skipped++
			response = &BillingUsageResponse{} // done marker only
		} else {
			if handled {
				login = current
				if response, err = client.FetchUserAICreditUsage(ctx, login, month); err != nil {
					return fmt.Errorf("billing user %d/%d after rename: %w", index+1, len(ids), err)
				}
			}
			if err := validateUserBilling(response, login, cfg.EnterpriseSlug, month); err != nil {
				return fmt.Errorf("billing user %d/%d: %w", index+1, len(ids), err)
			}
		}
		rows := billingRows(response, month, cfg.EnterpriseSlug, id, login)
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
	if err := publishBillingMonth(ctx, client, store, cfg, month, len(ids)); err != nil {
		return err
	}
	slog.Info("User billing month complete", "month", month.Format("2006-01"), "users", len(ids), "skipped_deleted", skipped)
	return nil
}
