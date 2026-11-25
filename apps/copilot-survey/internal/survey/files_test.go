package survey

import (
	"io/fs"
	"os"
)

func init() {
	SetFiles(os.DirFS("../..").(fs.ReadFileFS))
}
