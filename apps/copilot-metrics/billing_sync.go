package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"cloud.google.com/go/civil"
)

var billingSyncStart = time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)

func closedBillingMonths(now time.Time) []time.Time {
	now = now.UTC()
	current := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var months []time.Time
	for month := current.AddDate(0, -1, 0); !month.Before(billingSyncStart); month = month.AddDate(0, -1, 0) {
		months = append(months, month)
	}
	return months
}

type billingSyncFetcher interface {
	userBillingFetcher
	FetchUserID(context.Context, string) (string, error)
	FetchEnterpriseAICreditUsage(context.Context, time.Time) (*BillingUsageResponse, error)
}

type billingSyncStore interface {
	userBillingStore
	BillingSourceDays(context.Context, time.Time, string) (map[string]bool, error)
}

func stopBillingSync(err error) bool {
	var status *billingHTTPError
	return errors.Is(err, errBillingBudget) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		isTerminalGitHubError(err) ||
		(errors.As(err, &status) && (status.Status == 401 || status.Status == 403 || status.Status == 429))
}

func syncUserBilling(ctx context.Context, billing *BillingClient, gh *GitHubClient, store *BigQueryClient, cfg *Config, now time.Time) error {
	billing.budgeted, billing.remaining = true, 5000
	var failures []error
	// Newer months get a turn before an older backlog consumes the runtime budget.
	var pending []time.Time
	for _, month := range closedBillingMonths(now) {
		complete, err := store.UserBillingRunComplete(ctx, month, cfg.EnterpriseSlug)
		if err != nil {
			return err
		}
		if complete {
			continue
		}
		pending = append(pending, month)
	}
	for index, month := range pending {
		billing.requestLimit = billing.requests + (2000-billing.requests)/(len(pending)-index)
		deadline, ok := ctx.Deadline()
		if !ok {
			return fmt.Errorf("billing sync requires a runtime deadline")
		}
		monthCtx, cancel := context.WithTimeout(ctx, time.Until(deadline)/time.Duration(len(pending)-index))
		err := repairBillingSources(monthCtx, gh, store, cfg, month)
		if err == nil {
			err = syncBillingMonth(monthCtx, billing, store, cfg, month, userBillingPause)
		}
		cancel()
		if errors.Is(err, errBillingBudget) && billing.requests < 2000 && billing.remaining > 500 {
			if err != errBillingBudget {
				failures = append(failures, err)
			}
			slog.Info("Billing month request budget checkpointed", "month", month.Format("2006-01"))
			continue
		}
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			if err != context.DeadlineExceeded {
				failures = append(failures, err)
			}
			slog.Info("Billing month checkpointed", "month", month.Format("2006-01"))
			continue
		}
		if err != nil {
			if stopBillingSync(err) {
				if len(failures) > 0 {
					return errors.Join(append(failures, err)...)
				}
				return err
			}
			failures = append(failures, fmt.Errorf("month %s billing: %w", month.Format("2006-01"), err))
		}
	}
	return errors.Join(failures...)
}

func repairBillingSources(ctx context.Context, gh *GitHubClient, store interface {
	BillingSourceDays(context.Context, time.Time, string) (map[string]bool, error)
	ReplaceUserMetrics(context.Context, time.Time, *FetchResult) error
	ReplaceUserTeams(context.Context, time.Time, *FetchResult) error
}, cfg *Config, month time.Time) error {
	days, err := store.BillingSourceDays(ctx, month, cfg.EnterpriseSlug)
	if err != nil {
		return err
	}
	var failures []error
	for day := month; day.Before(month.AddDate(0, 1, 0)); day = day.AddDate(0, 0, 1) {
		for _, report := range []struct {
			table   string
			fetch   func(context.Context, time.Time) (*FetchResult, error)
			replace func(context.Context, time.Time, *FetchResult) error
		}{
			{cfg.BigQueryUserMetricsTable, gh.FetchDailyUserMetrics, store.ReplaceUserMetrics},
			{cfg.BigQueryUserTeamsTable, gh.FetchDailyUserTeams, store.ReplaceUserTeams},
		} {
			if days[day.Format("2006-01-02")+":"+report.table] {
				continue
			}
			result, err := report.fetch(ctx, day)
			if err == nil && (!result.Complete || result.Scope != "enterprise" || result.ScopeID != cfg.EnterpriseSlug) {
				err = fmt.Errorf("required enterprise report is incomplete")
			}
			if err == nil {
				err = report.replace(ctx, day, result)
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("%s %s: %w", report.table, day.Format("2006-01-02"), err))
				if stopBillingSync(err) {
					return errors.Join(failures...)
				}
				if ctx.Err() != nil {
					return errors.Join(append(failures, ctx.Err())...)
				}
			}
		}
	}
	return errors.Join(failures...)
}

func billingRows(response *BillingUsageResponse, month time.Time, scope, id, login string) []UserBillingRow {
	bySKU := map[string]UserBillingRow{}
	loaded := time.Now().UTC()
	for _, item := range response.UsageItems {
		if item.Product != "Copilot" || (item.SKU != "Copilot AI Credits" && item.SKU != "Copilot Cloud Agent") {
			continue
		}
		row := bySKU[item.SKU]
		row.Month, row.ScopeID, row.UserID, row.GitHubLogin, row.SKU, row.LoadedAt = civil.DateOf(month), scope, id, login, item.SKU, loaded
		row.GrossAmount += item.GrossAmount
		row.NetAmount += item.NetAmount
		bySKU[item.SKU] = row
	}
	rows := []UserBillingRow{{Month: civil.DateOf(month), ScopeID: scope, UserID: id, GitHubLogin: login, SKU: "done", LoadedAt: loaded}}
	for _, row := range bySKU {
		rows = append(rows, row)
	}
	return rows
}

func syncBillingMonth(ctx context.Context, fetcher billingSyncFetcher, store billingSyncStore, cfg *Config, month time.Time, pause time.Duration) error {
	users, err := store.GetBillingUsers(ctx, month, cfg.EnterpriseSlug)
	if err != nil {
		return err
	}
	if len(users) == 0 {
		return fmt.Errorf("no source users")
	}
	done, err := store.UserBillingDone(ctx, month, cfg.EnterpriseSlug)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(users))
	for id := range users {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	unresolved, skipped := 0, 0
	var failures []error
	for _, id := range ids {
		if done[id] {
			continue
		}
		if ctx.Err() != nil {
			if len(failures) > 0 {
				return errors.Join(append(failures, ctx.Err())...)
			}
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			if len(failures) > 0 {
				return errors.Join(append(failures, ctx.Err())...)
			}
			return ctx.Err()
		case <-time.After(pause):
		}
		currentID, err := fetcher.FetchUserID(ctx, users[id])
		if gone, _ := deletedAccount(ctx, fetcher, id, err); gone {
			skipped++
			if err := store.ReplaceUserBilling(ctx, billingRows(&BillingUsageResponse{}, month, cfg.EnterpriseSlug, id, users[id])); err != nil {
				return errors.Join(append(failures, err)...)
			}
			done[id] = true
			continue
		}
		if stopBillingSync(err) {
			if len(failures) > 0 {
				return errors.Join(append(failures, err)...)
			}
			return err
		}
		if err != nil || currentID != id {
			unresolved++
			failures = append(failures, fmt.Errorf("billing identity could not be verified"))
			slog.Warn("User billing identity unresolved", "month", month.Format("2006-01"))
			if err := store.ReplaceUserBilling(ctx, []UserBillingRow{{Month: civil.DateOf(month), ScopeID: cfg.EnterpriseSlug, UserID: id, GitHubLogin: users[id], SKU: "identity_unresolved", LoadedAt: time.Now().UTC()}}); err != nil {
				return errors.Join(append(failures, err)...)
			}
			continue
		}
		response, err := fetcher.FetchUserAICreditUsage(ctx, users[id], month)
		if err != nil {
			if stopBillingSync(err) {
				if len(failures) > 0 {
					return errors.Join(append(failures, err)...)
				}
				return err
			}
			unresolved++
			failures = append(failures, err)
			continue
		}
		if err := validateUserBilling(response, users[id], cfg.EnterpriseSlug, month); err != nil {
			unresolved++
			failures = append(failures, err)
			continue
		}
		if err := store.ReplaceUserBilling(ctx, billingRows(response, month, cfg.EnterpriseSlug, id, users[id])); err != nil {
			if len(failures) > 0 {
				return errors.Join(append(failures, err)...)
			}
			return err
		}
		done[id] = true
	}
	if unresolved > 0 {
		return fmt.Errorf("%d users remain unresolved: %w", unresolved, errors.Join(failures...))
	}
	// Source repair may add users between invocations. Never publish a stale census.
	latest, err := store.GetBillingUsers(ctx, month, cfg.EnterpriseSlug)
	if err != nil {
		return err
	}
	if len(latest) != len(users) {
		return fmt.Errorf("source users changed during collection")
	}
	for id, login := range latest {
		if !done[id] || users[id] != login {
			return fmt.Errorf("source users changed during collection")
		}
	}
	enterprise, err := fetcher.FetchEnterpriseAICreditUsage(ctx, month)
	if err != nil {
		return err
	}
	if err := validateUserBilling(enterprise, "", cfg.EnterpriseSlug, month); err != nil {
		return err
	}
	{
		days, err := store.BillingSourceDays(ctx, month, cfg.EnterpriseSlug)
		if err != nil {
			return err
		}
		for day := month; day.Before(month.AddDate(0, 1, 0)); day = day.AddDate(0, 0, 1) {
			for _, report := range []string{cfg.BigQueryUserMetricsTable, cfg.BigQueryUserTeamsTable} {
				if !days[day.Format("2006-01-02")+":"+report] {
					return fmt.Errorf("source coverage changed during collection")
				}
			}
		}
	}
	if err := store.ReplaceUserBilling(ctx, billingRows(enterprise, month, cfg.EnterpriseSlug, "", "")); err != nil {
		return err
	}
	if err := store.CompleteUserBilling(ctx, month, cfg.EnterpriseSlug, len(users)); err != nil {
		return err
	}
	slog.Info("User billing month complete", "month", month.Format("2006-01"), "users", len(users), "skipped_deleted", skipped)
	return nil
}
