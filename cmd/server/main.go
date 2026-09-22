// cmd/server/main.go —— 服务入口
//
// 职责：加载配置 → 打开 SQLite → 启动 Hub 事件循环 → 注册 HTTP 路由 → 优雅退出
// 本文件不写任何业务逻辑。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-ws-sqlite/internal/config"
	"go-ws-sqlite/internal/hub"
	"go-ws-sqlite/internal/store"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel(),
	}))
	slog.SetDefault(logger)

	slog.Info("starting go-ws-sqlite",
		"port", cfg.AppPort,
		"mode", cfg.AppMode,
		"db", cfg.DBPath,
	)

	// ---------- 1. 打开 SQLite ----------
	db, err := store.Open(cfg.DBPath, cfg.DBBusyTimeout)
	if err != nil {
		slog.Error("open sqlite failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := store.Migrate(db); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
	slog.Info("sqlite ready", "path", cfg.DBPath)

	// ---------- 2. 启动 Hub 事件循环 ----------
	h := hub.New(db, cfg)
	go h.Run()
	defer h.Stop()

	// ---------- 3. HTTP 路由 ----------
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz(h))

	addr := ":" + cfg.AppPort
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// ---------- 4. 启动 + 优雅退出 ----------
	go func() {
		slog.Info("listening", "addr", "http://127.0.0.1"+addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	slog.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	slog.Info("bye")
}

// handleHealthz —— Phase 1 占位接口
//
// 后续将扩展为：连接数 / 频道数 / 最近消息时间戳 / 数据库状态。
func handleHealthz(h *hub.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"service":"go-ws-sqlite","phase":"M1-skeleton"}`))
	}
}