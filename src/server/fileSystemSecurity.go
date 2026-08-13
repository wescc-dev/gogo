package server

import (
	"fmt"
	"os"
	"strings"
)

func AssertFileSystemAccess(path string) error {
	var info, err = os.Stat(path)
	if err != nil {
		return fmt.Errorf("error accessing file system.%s; %s", path, err)
	}
	name := info.Name()
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("disallowed file name: %s", name)
	}
	return nil
}
