package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stage-clearance/internal/application"
	"stage-clearance/internal/httpui"
	"stage-clearance/internal/rules"
	"stage-clearance/internal/store"
)

type Runtime struct {
	Config  Config
	Store   *store.FileStore
	Service *application.Service
	Server  *http.Server
	Logger  *slog.Logger
}

func BuildRuntime(config Config, logger *slog.Logger) (*Runtime, error) {
	if logger == nil {
		logger = slog.Default()
	}
	fileStore, err := store.New(config.DataDir)
	if err != nil {
		return nil, fmt.Errorf("初始化本地存储: %w", err)
	}
	if err := fileStore.CheckWritable(); err != nil {
		return nil, err
	}
	engine := rules.NewDefaultEngine()
	service := application.NewService(fileStore, fileStore, engine, application.SystemClock{}, application.RandomIDGenerator{})
	handler := httpui.NewHandler(service, logger)
	server := &http.Server{
		Addr: config.Address, Handler: handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: config.ReadTimeout,
		WriteTimeout: config.WriteTimeout, IdleTimeout: config.IdleTimeout,
	}
	return &Runtime{Config: config, Store: fileStore, Service: service, Server: server, Logger: logger}, nil
}

func (r *Runtime) Serve(ctx context.Context) error {
	listener, err := net.Listen("tcp", r.Config.Address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", r.Config.Address, err)
	}
	r.Logger.Info("舞台机械安全放行服务已启动", "address", listener.Addr().String(), "data_dir", r.Config.DataDir)
	errCh := make(chan error, 1)
	go func() {
		errCh <- r.Server.Serve(listener)
	}()
	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := r.Server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("关闭 HTTP 服务: %w", err)
		}
		err := <-errCh
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

func runProduction(config Config, logger *slog.Logger) error {
	runtime, err := BuildRuntime(config, logger)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runtime.Serve(ctx)
}
