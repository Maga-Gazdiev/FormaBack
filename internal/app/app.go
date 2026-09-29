package app

import (
	"context"
	"converter/internal/config"
	handler "converter/internal/handler/conversion"
	"converter/internal/handler/middleware"
	"converter/internal/infrastructure/converter"
	"converter/internal/repository/files"
	service "converter/internal/service/conversion"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func NewHandler(cfg config.Config) (http.Handler, error) {
	repository, err := files.NewRepository(cfg.TempDir)
	if err != nil {
		return nil, err
	}
	converterService := service.NewService(repository, converter.NewRunner(), converter.Routes(), cfg.MaxFileSize, cfg.ConversionTimeout)
	conversionHandler := handler.NewHandler(converterService, cfg.MaxConcurrent)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, conversionHandler)
	return middleware.CORS(cfg.CORSOrigins, mux), nil
}
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	handler, err := NewHandler(cfg)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: cfg.Address(), Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: cfg.ConversionTimeout + 30*time.Second, WriteTimeout: cfg.ConversionTimeout + 30*time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("Converter API started", "address", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		slog.Info("Shutting down Converter API")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ConversionTimeout+15*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
