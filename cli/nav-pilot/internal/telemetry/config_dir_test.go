package telemetry

import (
	"path/filepath"
	"strings"
	"testing"
)

// NAV_PILOT_CONFIG navngir konfigurasjonsfila, og katalogen dens er der
// nav-pilot sin øvrige egen tilstand hører hjemme. Leste denne $HOME direkte,
// skrev en kjøring med isolert config likevel device-id inn i utviklerens
// ekte hjemmekatalog (#627).
func TestConfigDirFollowsNavPilotConfig(t *testing.T) {
	iso := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(iso, "config.toml"))

	dir, err := GetConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != iso {
		t.Errorf("config-katalogen ble %q, ventet %q", dir, iso)
	}

	id, err := GetOrCreateDeviceID()
	if err != nil {
		t.Fatalf("device-id: %v", err)
	}
	if strings.TrimSpace(id) == "" {
		t.Error("device-id ble tomt")
	}
	// Og fila skal ligge der, ikke i hjemmekatalogen.
	if _, err := filepath.Rel(iso, filepath.Join(dir, "device-id")); err != nil {
		t.Errorf("device-id havnet utenfor den isolerte katalogen: %v", err)
	}
}

// Kontroll: uten variabelen skal den fortsatt falle tilbake på hjemmekatalogen,
// ellers ville testen passert på at alt peker til tmp.
func TestConfigDirFallsBackToHome(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := GetConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".nav-pilot"); dir != want {
		t.Errorf("config-katalogen ble %q, ventet %q", dir, want)
	}
}
