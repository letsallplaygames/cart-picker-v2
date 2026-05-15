package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"pickcart/internal/cache"
	"pickcart/internal/config"
	"pickcart/internal/hardware"
	"pickcart/internal/led"
	"pickcart/internal/odoo"
	"pickcart/internal/picker"
	"pickcart/internal/ui"
)

const (
	cacheDirName  = "data"
	cacheFileName = "api_cache.db"
	logFileName   = "app.log"
)

func main() {
	cartNumber := flag.Int("cart", 1, "cart number")
	profileName := flag.String("profile", "standard", fmt.Sprintf("cart profile (%s)", strings.Join(config.AvailableProfiles(), ", ")))
	clearCache := flag.Bool("clear-cache", false, "clear cached API responses before startup")
	flag.Parse()

	logger, logFile, err := setupLogging(logFileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configure logging: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	slog.SetDefault(logger)

	cfg, err := config.Load(*cartNumber, *profileName)
	if err != nil {
		slog.Error("failed to load app config", "error", err)
		os.Exit(1)
	}

	profile, err := config.GetCartProfile(cfg.ProfileName)
	if err != nil {
		slog.Error("failed to resolve cart profile", "error", err)
		os.Exit(1)
	}

	hardwareResult := hardware.Check()
	if hardwareResult.Passed {
		slog.Info("hardware check passed", "system", hardwareResult.System, "arch", hardwareResult.Arch, "gpio", hardwareResult.GPIODevice, "library", hardwareResult.WS281xLibraryPath)
	} else {
		slog.Warn("hardware check failed; running in simulation mode", "system", hardwareResult.System, "arch", hardwareResult.Arch)
		for _, message := range hardwareResult.Messages {
			slog.Warn(message)
		}
	}

	if strings.TrimSpace(cfg.OdooAPIKey) == "" || strings.TrimSpace(cfg.OdooBaseURL) == "" {
		slog.Error("missing required Odoo configuration", "required", []string{"ODOO_API_KEY", "ODOO_BASE_URL"})
		os.Exit(1)
	}

	cachePath := filepath.Join(cacheDirName, cacheFileName)
	if *clearCache {
		if err := clearCacheFile(cachePath); err != nil {
			slog.Error("failed to clear cache", "error", err, "path", cachePath)
			os.Exit(1)
		}
		slog.Info("cache cleared", "path", cachePath)
	}

	apiCache, err := cache.New(cachePath)
	if err != nil {
		slog.Error("failed to create cache", "error", err, "path", cachePath)
		os.Exit(1)
	}
	defer func() {
		if err := apiCache.Close(); err != nil {
			slog.Warn("failed to close cache", "error", err)
		}
	}()

	client := odoo.New(odoo.Config{
		APIKey:   cfg.OdooAPIKey,
		BaseURL:  cfg.OdooBaseURL,
		Database: cfg.OdooDB,
		Cache:    apiCache,
		UseCache: true,
	})
	provider := odoo.NewProvider(client)
	pick := picker.New(provider)
	ledController := led.New(cfg.CartNumber, profile.Name)
	defer ledController.Cleanup()

	application := ui.NewApp(cfg, profile, pick, ledController)
	slog.Info("starting pickcart", "cart", cfg.CartNumber, "profile", profile.Name)
	application.Run()
}

func setupLogging(logPath string) (*slog.Logger, *os.File, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}

	writer := io.MultiWriter(os.Stderr, logFile)
	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(handler), logFile, nil
}

func clearCacheFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
