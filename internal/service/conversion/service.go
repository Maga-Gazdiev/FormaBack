package conversion

import (
	"context"
	"converter/internal/model"
	"converter/internal/repository/files"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Converter interface {
	Convert(context.Context, string, string, string, string, string, string) error
}
type Service struct {
	repository  files.Repository
	converter   Converter
	routes      []model.Route
	maxFileSize int64
	timeout     time.Duration
}
type Result struct {
	Path    string
	Name    string
	Cleanup func()
}

func NewService(repository files.Repository, converter Converter, routes []model.Route, maxFileSize int64, timeout time.Duration) *Service {
	return &Service{repository: repository, converter: converter, routes: routes, maxFileSize: maxFileSize, timeout: timeout}
}
func (s *Service) Formats() []model.Route { return s.routes }
func (s *Service) MaxFileSize() int64     { return s.maxFileSize }
func (s *Service) Convert(ctx context.Context, name, target string, reader io.Reader) (result *Result, err error) {
	from := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	target = strings.ToLower(target)
	engine := ""
	for _, route := range s.routes {
		if route.From == from {
			for _, to := range route.To {
				if to == target {
					engine = route.Engine
				}
			}
		}
	}
	if engine == "" {
		return nil, model.ErrUnsupported
	}
	if engine == "native" && from != "csv" && from != "json" && target != "png" && target != "jpg" && target != "gif" {
		engine = "convert"
	}
	workspace, err := s.repository.Create()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			workspace.Cleanup()
		}
	}()
	input, err := workspace.Save(reader, from, s.maxFileSize)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	output := filepath.Join(workspace.Dir, "result."+target)
	if err = s.converter.Convert(ctx, engine, input, output, from, target, workspace.Dir); err != nil {
		return nil, fmt.Errorf("%w: %s -> %s: %v", model.ErrConversion, from, target, err)
	}
	filename := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)) + "." + target
	if _, zipErr := os.Stat(output + ".zip"); zipErr == nil {
		output += ".zip"
		filename += ".zip"
	}
	if _, err = os.Stat(output); err != nil {
		return nil, fmt.Errorf("%w: result missing: %v", model.ErrConversion, err)
	}
	return &Result{Path: output, Name: filename, Cleanup: workspace.Cleanup}, nil
}

func (s *Service) Timeout() time.Duration { return s.timeout }
