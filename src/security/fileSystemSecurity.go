package security

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/wescc-dev/gogo/src/configuration"
)

type fileAccessConfig struct {
	Enabled       bool     `json:"enabled"`
	ExcludedPaths []string `json:"excludedPaths"`
}

var cfg = configuration.GetConfiguration()
var accessConfig fileAccessConfig
var compiled []*regexp.Regexp

func init() {
	err := loadConfig(cfg.FileAccessConfigFile)
	if err != nil {
		log.Fatal(err)
	}
}
func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &accessConfig); err != nil {
		return err
	}

	compiled = make([]*regexp.Regexp, 0, len(accessConfig.ExcludedPaths))
	for _, pattern := range accessConfig.ExcludedPaths {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex %q: %w", pattern, err)
		}
		compiled = append(compiled, re)
	}

	return nil
}

func AssertFileSystemAccess(path string) error {
	if !accessConfig.Enabled {
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

		dbase := filepath.Base(dir)
		if isExcluded(dbase) {
			return fmt.Errorf("disallowed directory in path: %s", dbase)
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
