package conversion

import (
	"context"
	"converter/internal/model"
	"converter/internal/repository/files"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeConverter struct{ fail bool }

func (f fakeConverter) Convert(ctx context.Context, engine, input, output, from, to, dir string) error {
	if f.fail {
		return errors.New("converter failed")
	}
	return os.WriteFile(output, []byte("converted"), 0600)
}
func TestWorkspaceLifecycle(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			root := t.TempDir()
			repo, err := files.NewRepository(root)
			if err != nil {
				t.Fatal(err)
			}
			service := NewService(repo, fakeConverter{fail: fail}, []model.Route{{From: "txt", To: []string{"pdf"}, Engine: "fake"}}, 1024, time.Second)
			result, err := service.Convert(context.Background(), "test.txt", "pdf", strings.NewReader("hello"))
			if fail {
				if !errors.Is(err, model.ErrConversion) {
					t.Fatal(err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if result.Name != "test.pdf" {
					t.Fatal(result.Name)
				}
				if _, err := os.Stat(result.Path); err != nil {
					t.Fatal(err)
				}
				result.Cleanup()
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("temporary files remain: %v %v", entries, err)
			}
		})
	}
}
func TestOversizeAndUnsupported(t *testing.T) {
	root := t.TempDir()
	repo, _ := files.NewRepository(root)
	service := NewService(repo, fakeConverter{}, []model.Route{{From: "txt", To: []string{"pdf"}, Engine: "fake"}}, 3, time.Second)
	_, err := service.Convert(context.Background(), "x.txt", "pdf", strings.NewReader("large"))
	if !errors.Is(err, model.ErrTooLarge) {
		t.Fatal(err)
	}
	_, err = service.Convert(context.Background(), "x.txt", "../../out", strings.NewReader("ok"))
	if !errors.Is(err, model.ErrUnsupported) {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("temporary files leaked")
	}
}
