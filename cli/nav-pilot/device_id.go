package main

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// getOrCreateDeviceID returns a stable, deterministic device identifier.
// It generates a SHA256 hash of hostname + CLI executable path + MAC address.
// The result is stored in ~/.nav-pilot/device-id for persistence.
// This UUID is:
// - Stable: Same machine always produces the same UUID
// - Deterministic: Not random, based on hardware + install path
// - Private: Never contains username, email, or identifiable data
func getOrCreateDeviceID() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config dir: %w", err)
	}

	idFile := filepath.Join(configDir, "device-id")

	// If file exists, read and return it
	if data, err := os.ReadFile(idFile); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}

	// Otherwise: generate deterministic ID from hardware
	deviceID, err := generateDeterministicDeviceID()
	if err != nil {
		return "", fmt.Errorf("failed to generate device ID: %w", err)
	}

	// Write to disk for future use (with restrictive permissions)
	if err := os.WriteFile(idFile, []byte(deviceID+"\n"), 0600); err != nil {
		// Non-fatal: we can still use the in-memory ID
		debugLog("failed to persist device ID to %s: %v", idFile, err)
	}

	return deviceID, nil
}

// generateDeterministicDeviceID creates a SHA256 hash of:
// - hostname (machine identity)
// - executable path (installation path / env)
// - MAC address (hardware identity, if available)
// Result format: "nav-pilot-" + first 12 hex chars of hash
func generateDeterministicDeviceID() (string, error) {
	var components []string

	// Component 1: Hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	components = append(components, hostname)

	// Component 2: CLI executable path
	execPath, err := os.Executable()
	if err != nil {
		execPath = "unknown"
	}
	components = append(components, execPath)

	// Component 3: MAC address (first one available, if any)
	macAddr, err := getMACAddress()
	if err != nil {
		macAddr = "unknown"
	}
	components = append(components, macAddr)

	// Create deterministic input string
	input := strings.Join(components, "|")

	// Hash it
	hash := sha256.Sum256([]byte(input))
	hashHex := fmt.Sprintf("%x", hash[:])

	// Return as "nav-pilot-{first12chars}"
	deviceID := fmt.Sprintf("nav-pilot-%s", hashHex[:12])

	return deviceID, nil
}

// getMACAddress returns the first MAC address found on the system.
// Used to make device ID stable across reinstalls/path changes.
// Returns "unknown" if unable to determine.
func getMACAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "unknown", err
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		// Skip interfaces with no MAC address
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		return iface.HardwareAddr.String(), nil
	}

	return "unknown", nil
}

// getConfigDir returns the nav-pilot config directory, creating it if needed.
// Uses: ~/.nav-pilot/
func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".nav-pilot")

	// Create if doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create config dir %s: %w", configDir, err)
	}

	return configDir, nil
}

// debugLog logs a message if DEBUG is set. Used for non-critical issues.
func debugLog(format string, args ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}
