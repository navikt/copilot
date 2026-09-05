//go:build !linux

package cli

// strictPresetSupported reports whether cplt's strict preset can work here.
//
// Only Linux has a gate: cplt enforces `proxy.forced` there with Landlock,
// which needs ABI v4, and refuses to launch below it. macOS enforces the same
// thing with Seatbelt, which has no equivalent floor. See strict_gate_linux.go.
var strictPresetSupported = func() (bool, string) { return true, "" }
