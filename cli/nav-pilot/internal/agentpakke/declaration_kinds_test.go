package agentpakke_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// DeclaredItemTypes speiler CLI-ens artefakttyper, men lista står for seg selv
// fordi erklæringa leses før noen resolver finnes. #647 la til en femte type
// uten å røre den, og et repo som installerte en pakke med hooks kom ikke forbi
// sin egen erklæring (#649).
//
// Testen henter typene fra source.AllKinds i stedet for å skrive dem opp på
// nytt. En liste skrevet to steder er nettopp det som glir fra hverandre, og en
// test som gjentar den arver feilen den skal fange. Den ligger i
// agentpakke_test fordi source importerer agentpakke.
func TestEveryArtifactKindIsDeclarable(t *testing.T) {
	for _, kind := range source.AllKinds {
		root := t.TempDir()
		dir := filepath.Join(root, ".nav-pilot")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(map[string]any{
			"contractVersion": "1",
			"source":          "navikt/grillmester",
			"items":           map[string]string{"noe": kind.Name},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "agentpakke.lock.json"), body, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := agentpakke.LoadDeclaration(root); err != nil {
			t.Errorf("artefakttypen %q kan ikke stå i en erklæring: %v", kind.Name, err)
		}
	}
}
