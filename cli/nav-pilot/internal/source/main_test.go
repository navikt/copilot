package source

import (
	"os"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

func TestMain(m *testing.M) { os.Exit(testhome.Run(m)) }
