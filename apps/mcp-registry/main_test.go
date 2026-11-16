package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestConnectWithRetry_RetriesUntilSuccess(t *testing.T) {
	calls := 0
	want := &DB{}
	got, err := connectWithRetry(context.Background(), func(ctx context.Context) (*DB, error) {
		calls++
		if _, ok := ctx.Deadline(); !ok {
			t.Error("attempt has no deadline")
		}
		if calls < 3 {
			return nil, errors.New("down")
		}
		return want, nil
	}, time.Second, time.Millisecond, 2*time.Millisecond)
	if err != nil || got != want || calls != 3 {
		t.Fatalf("got %v, %v after %d calls", got, err, calls)
	}
}

func TestConnectWithRetry_StopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_, err := connectWithRetry(ctx, func(context.Context) (*DB, error) {
		cancel()
		return nil, errors.New("down")
	}, time.Second, time.Hour, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
