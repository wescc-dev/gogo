package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func AssertFileSystemAccess(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error accessing file system: %s; %w", path, err)
	}

	// Reject if the file itself begins with "."
	if strings.HasPrefix(info.Name(), ".") {
		return fmt.Errorf("disallowed file name: %s", info.Name())
	}

	// Walk up the directory tree
	dir := filepath.Dir(path)

	for {
		base := filepath.Base(dir)

		// Reject directories that begin with "."
		if strings.HasPrefix(base, ".") {
			return fmt.Errorf("disallowed directory in path: %s", base)
		}

		// Stop at filesystem root
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return nil
}
