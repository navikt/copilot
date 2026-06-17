package cli

import "github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"

func setupTestCache(t interface {
	Helper()
	TempDir() string
	Cleanup(func())
}) {
	t.Helper()
	dir := t.TempDir()
	origHome := artifacts.CacheHome
	artifacts.CacheHome = dir
	t.Cleanup(func() { artifacts.CacheHome = origHome })
}
