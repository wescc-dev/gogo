package security

import (
	"encoding/json"
	"fmt"
	"gogopher/src/configuration"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

type FileAccessConfig struct {
	Enabled       bool     `json:"enabled"`
	ExcludedPaths []string `json:"excludedPaths"`
}

var cfg = configuration.GetConfiguration()
var fileAccessConfig FileAccessConfig
var compiled []*regexp.Regexp

func init() {
	err := loadConfig(cfg.FireWallConfigFile)
	if err != nil {
		log.Fatal(err)
	}
}
func loadConfig(path string) error {
	if compiled != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &fileAccessConfig); err != nil {
		return err
	}

	compiled = make([]*regexp.Regexp, 0, len(fileAccessConfig.ExcludedPaths))
	for _, pattern := range fileAccessConfig.ExcludedPaths {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex %q: %w", pattern, err)
		}
		compiled = append(compiled, re)
	}

	return nil
}

func AssertFileSystemAccess(path string) error {
	if !fileAccessConfig.Enabled {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error accessing file system: %s; %w", path, err)
	}

	// Check the file name itself
	if isExcluded(info.Name()) {
		return fmt.Errorf("disallowed file name: %s", info.Name())
	}

	// Walk up the directory tree
	dir := filepath.Dir(path)

	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		base := filepath.Base(dir)

		if isExcluded(base) {
			return fmt.Errorf("disallowed directory in path: %s", base)
		}

		dir = parent
	}

	return nil
}

func isExcluded(name string) bool {
	for _, re := range compiled {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}
