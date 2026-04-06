package exports

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileStore manages export files on disk.
type FileStore struct {
	dir string
}

// NewFileStore creates a FileStore, ensuring the directory exists.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create export dir %s: %w", dir, err)
	}
	return &FileStore{dir: dir}, nil
}

// Write persists export bytes to disk and returns the file path.
// The filename is <exportID><extension>.
func (s *FileStore) Write(exportID string, format Format, data []byte) (string, error) {
	name := exportID + format.Extension()
	path := filepath.Join(s.dir, name)
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return "", fmt.Errorf("write export file: %w", err)
	}
	return path, nil
}

// Read returns the bytes of a previously written export file.
func (s *FileStore) Read(filePath string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read export file: %w", err)
	}
	return data, nil
}
