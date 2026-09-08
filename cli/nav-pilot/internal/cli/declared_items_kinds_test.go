package cli

import (
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// Et utvalg i erklæringa må kunne navngi enhver artefakttype pakka sender.
// applyDeclaredItems hadde sin egen liste over typer, og #739 la til extension
// uten å røre den, slik #647 gjorde med hook i tre andre lister (#649, #650,
// #708). Typene hentes fra source.AllKinds, ikke skrevet opp på nytt: en test
// som gjentar lista arver feilen den skal fange.
func TestDeclaredItemsAcceptsEveryKind(t *testing.T) {
	for _, kind := range source.AllKinds {
		m := &Manifest{Name: "testpakke"}
		if !m.SetNamesByKind(kind, []string{"noe"}) {
			t.Fatalf("manifestet kjenner ikke typen %q", kind.Name)
		}
		got, err := applyDeclaredItems(m, map[string]string{"noe": kind.Name})
		if err != nil {
			t.Errorf("artefakttypen %q kan ikke velges i en erklæring: %v", kind.Name, err)
			continue
		}
		names, _ := got.NamesByKind(kind)
		if len(names) != 1 || names[0] != "noe" {
			t.Errorf("typen %q: utvalget ga %v, ventet [noe]", kind.Name, names)
		}
	}
}
