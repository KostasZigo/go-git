package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/KostasZigo/gogit/internal/constants"
)

// FindRoot locates the repository root by walking up the directory tree
// from the current working directory until a .gogit directory is found.
// Returns an error if the filesystem root is reached without finding one.
func FindRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		gogitPath := filepath.Join(dir, constants.Gogit)
		if info, err := os.Stat(gogitPath); err == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%s directory not found", constants.Gogit)
		}
		dir = parent
	}
}
