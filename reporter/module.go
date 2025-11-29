package reporter

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/mod/modfile"
)

func GetModuleName(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", errors.WithStack(err)
	}

	mod, err := modfile.Parse("", b, nil)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if mod != nil && mod.Module != nil {
		return mod.Module.Mod.Path, nil
	}

	return "", nil
}
