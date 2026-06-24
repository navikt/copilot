package provider

import (
	"os"
	"os/exec"
	"strings"
)

const navPilotUseRTKEnv = "NAV_PILOT_USE_RTK"

const (
	rtkResultApplied        = "applied"
	rtkResultNotEnabled     = "not_enabled"
	rtkResultNonInteractive = "non_interactive"
	rtkResultMissing        = "rtk_missing"
)

type rtkSetting int

const (
	rtkSettingUnset rtkSetting = iota
	rtkSettingEnabled
)

type rtkDeps struct {
	getenv        func(string) string
	lookPath      func(string) (string, error)
	isInteractive func() bool
}

func defaultRTKDeps() rtkDeps {
	return rtkDeps{
		getenv:        os.Getenv,
		lookPath:      exec.LookPath,
		isInteractive: isInteractiveSession,
	}
}

func parseRTKSetting(force bool, getenv func(string) string) rtkSetting {
	if force {
		return rtkSettingEnabled
	}
	v := strings.TrimSpace(getenv(navPilotUseRTKEnv))
	if v == "" {
		return rtkSettingUnset
	}
	return rtkSettingEnabled
}

func rtkWrappedCommandWithDeps(cmdPath string, args []string, useRTK bool, deps rtkDeps) (string, []string, string) {
	switch parseRTKSetting(useRTK, deps.getenv) {
	case rtkSettingUnset:
		return cmdPath, args, rtkResultNotEnabled
	}
	if !deps.isInteractive() {
		return cmdPath, args, rtkResultNonInteractive
	}

	rtkPath, err := deps.lookPath("rtk")
	if err != nil || strings.TrimSpace(rtkPath) == "" {
		return cmdPath, args, rtkResultMissing
	}

	wrappedArgs := append([]string{cmdPath}, args...)
	return rtkPath, wrappedArgs, rtkResultApplied
}

func isInteractiveSession() bool {
	return isTerminalDevice(os.Stdin) && isTerminalDevice(os.Stdout) && isTerminalDevice(os.Stderr)
}

func isTerminalDevice(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
