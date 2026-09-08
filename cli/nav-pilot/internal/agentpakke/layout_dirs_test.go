package agentpakke

import (
	"reflect"
	"strings"
	"testing"
)

// Layout.Dirs må dekke hvert felt i Layout. Stireglene og innholdssjekken gikk
// hver sin håndskrevne liste, og ingen av dem fikk layout.extensions da typen
// kom til (#739): en extensions-sti som pekte ut av repoet validerte grønt,
// mens den samme stien under layout.agents ble avvist.
//
// Testen leser feltene ut av structen i stedet for å skrive dem opp på nytt.
// En liste gjentatt i en test arver feilen den skal fange.
func TestLayoutDirsCoversEveryField(t *testing.T) {
	typ := reflect.TypeOf(Layout{})
	dirs := (&Layout{}).Dirs()
	if len(dirs) != typ.NumField() {
		t.Fatalf("Dirs dekker %d felt, Layout har %d", len(dirs), typ.NumField())
	}
	covered := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		covered[strings.TrimPrefix(d.Field, "layout.")] = true
	}
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if !covered[name] {
			t.Errorf("layout.%s valideres ikke: legg feltet til i Layout.Dirs", name)
		}
	}
}

// Dirs skal returnere verdiene, ikke bare navnene.
func TestLayoutDirsCarriesValues(t *testing.T) {
	l := &Layout{Agents: "a", Skills: "s", Extensions: "e"}
	got := map[string]string{}
	for _, d := range l.Dirs() {
		got[d.Field] = d.Value
	}
	for field, want := range map[string]string{"layout.agents": "a", "layout.skills": "s", "layout.extensions": "e"} {
		if got[field] != want {
			t.Errorf("%s = %q, ventet %q", field, got[field], want)
		}
	}
}
