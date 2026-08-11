package utility

import "os"

func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
