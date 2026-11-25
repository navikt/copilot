package store

import (
	"io/fs"
	"os"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

func init() {
	survey.SetFiles(os.DirFS("../..").(fs.ReadFileFS))
}
