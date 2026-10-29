package store

import "os"

func mkdir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}
