//go:build linux

package cli

import (
	"strconv"

	"golang.org/x/sys/unix"
)

// Whether cplt's strict preset can work on this machine.
//
// `strict` turns on `proxy.forced`, and on Linux cplt enforces forced egress
// with Landlock's kernel-level TCP-connect restriction. That needs Landlock ABI
// v4 or newer (kernel 6.7+). Below it cplt does not degrade — it refuses to
// launch, by design, because the alternative is open networking under a preset
// whose whole point is that egress is closed (cplt src/sandbox_landlock.rs,
// `check_proxy_forced_enforceable`).
//
// So on an older kernel, recommending strict does not weaken a session. It
// stops every session on the machine. That is worse than the problem the
// recommendation solves, so nav-pilot does not make it there.

// landlockABIVersion asks the kernel for the highest Landlock ABI it supports,
// returning 0 when Landlock is unavailable.
//
// This is the same probe cplt makes, deliberately: one
// `landlock_create_ruleset(NULL, 0, LANDLOCK_CREATE_RULESET_VERSION)`, which
// returns the version instead of creating a ruleset. It allocates no file
// descriptor and has no side effects.
//
// Asking the kernel beats reading `uname`. A kernel version is a proxy for the
// ABI, and a wrong one in the optimistic direction is exactly the failure being
// avoided: Landlock can be compiled out or disabled at boot with `lsm=`, and a
// 6.7+ host with Landlock off would pass a version check and then refuse to
// launch. It also beats reading /sys/kernel/security/landlock/abi_version,
// which is root-only on some hosts and would under-report.
func landlockABIVersion() int {
	// LANDLOCK_CREATE_RULESET_VERSION == 1.
	r, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, 1)
	if errno != 0 {
		return 0
	}
	return int(r)
}

// minLandlockABIForForcedProxy is the ABI cplt requires to enforce forced
// egress: v4, first in kernel 6.7.
const minLandlockABIForForcedProxy = 4

// strictPresetSupported reports whether cplt's strict preset can work here.
// A var for the same reason cpltSandboxPreset is one: tests cannot arrange a
// kernel, and the gate has to be exercised from both sides.
var strictPresetSupported = func() (bool, string) {
	if abi := landlockABIVersion(); abi < minLandlockABIForForcedProxy {
		// The reason states the fact only. Each caller adds its own
		// consequence, because "so it is not recommended" is wrong to print to
		// someone who already has strict set.
		return false, "this kernel cannot enforce cplt's forced-proxy egress, " +
			"so cplt refuses to launch under strict (Landlock ABI v" + strconv.Itoa(abi) +
			", needs v4: kernel 6.7+ with Landlock enabled)"
	}
	return true, ""
}
