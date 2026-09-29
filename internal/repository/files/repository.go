package files

import (
	"converter/internal/model"
	"io"
	"os"
	"path/filepath"
)

// Repository creates isolated workspaces for a single conversion.
type Repository interface{ Create() (*Workspace, error) }
type FileRepository struct{ root string }
type Workspace struct{ Dir string }

func NewRepository(root string) (*FileRepository, error) {
	if root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			return nil, err
		}
	}
	return &FileRepository{root: root}, nil
}
func (r *FileRepository) Create() (*Workspace, error) {
	dir, err := os.MkdirTemp(r.root, "converter-")
	if err != nil {
		return nil, err
	}
	return &Workspace{Dir: dir}, nil
}
func (w *Workspace) Save(reader io.Reader, extension string, limit int64) (string, error) {
	path := filepath.Join(w.Dir, "input."+extension)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	n, copyErr := io.Copy(f, io.LimitReader(reader, limit+1))
	closeErr := f.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if n > limit {
		return "", model.ErrTooLarge
	}
	if n == 0 {
		return "", model.ErrInvalid
	}
	return path, nil
}
func (w *Workspace) Cleanup() { _ = os.RemoveAll(w.Dir) }
