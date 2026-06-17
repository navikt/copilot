package provider

// cliVersion is the running binary version, injected by package main at startup.
// Used when resolving sources (e.g. for ensureOpenCodeNavContext).
var cliVersion = "dev"

// SetVersion sets the running CLI version. Called once from main().
func SetVersion(v string) { cliVersion = v }
