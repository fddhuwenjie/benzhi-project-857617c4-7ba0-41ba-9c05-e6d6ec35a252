package main

import (
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	config, err := ParseConfig(os.Args[1:], os.Getenv)
	if err != nil {
		logger.Error("配置无效", "error", err)
		os.Exit(2)
	}
	if config.SelfTest {
		err = runSelfTest(config, logger)
	} else {
		err = runProduction(config, logger)
	}
	if err != nil {
		logger.Error("服务退出", "error", err)
		os.Exit(1)
	}
}
