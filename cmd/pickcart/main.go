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

	// Expired responses are purged at startup so cached product images cannot
	// grow the cache file without bound. The window stays well past the 24h API
	// and 4h inventory TTLs because the stale inventory reads are the offline
	// fallback when Odoo is unreachable. Must never be 0, which wipes the cache.
	cacheRetentionHours = 24 * 7
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

	cartFlagSet := false
	profileFlagSet := false
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "cart":
			cartFlagSet = true
		case "profile":
			profileFlagSet = true
		}
	})

	var apiCache *cache.Cache
	var ledController *led.Controller
	ui.RunLauncher(ui.LaunchSettings{
		CartNumber:      *cartNumber,
		ProfileName:     *profileName,
		CartFromFlag:    cartFlagSet,
		ProfileFromFlag: profileFlagSet,
	}, func(settings ui.LaunchSettings) (*ui.App, error) {
		if ledController != nil {
			ledController.Cleanup()
			ledController = nil
		}
		if apiCache != nil {
			if err := apiCache.Close(); err != nil {
				slog.Warn("failed to close cache", "error", err)
			}
			apiCache = nil
		}

		cfg, err := config.Load(settings.CartNumber, settings.ProfileName)
		if err != nil {
			return nil, fmt.Errorf("load app config: %w", err)
		}

		profile, err := config.GetCartProfile(cfg.ProfileName)
		if err != nil {
			return nil, fmt.Errorf("resolve cart profile: %w", err)
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
			return nil, fmt.Errorf("missing required Odoo configuration: ODOO_API_KEY and ODOO_BASE_URL")
		}

		cachePath := filepath.Join(cacheDirName, cacheFileName)
		if *clearCache {
			if err := clearCacheFile(cachePath); err != nil {
				return nil, fmt.Errorf("clear cache: %w", err)
			}
			slog.Info("cache cleared", "path", cachePath)
		}

		apiCache, err = cache.New(cachePath)
		if err != nil {
			return nil, fmt.Errorf("create cache: %w", err)
		}

		if err := apiCache.Clear(cacheRetentionHours); err != nil {
			slog.Warn("failed to purge expired cache entries", "error", err, "retention_hours", cacheRetentionHours)
		}

		client := odoo.New(odoo.Config{
			APIKey:   cfg.OdooAPIKey,
			BaseURL:  cfg.OdooBaseURL,
			Database: cfg.OdooDB,
			Cache:    apiCache,
			UseCache: true,
		})
		provider := odoo.NewProvider(client)
		pick := picker.New(provider)
		ledController = led.New(cfg.CartNumber, profile.Name)

		slog.Info("starting pickcart", "cart", cfg.CartNumber, "profile", profile.Name)
		return ui.NewApp(cfg, profile, pick, ledController), nil
	})

	if ledController != nil {
		ledController.Cleanup()
	}
	if apiCache != nil {
		if err := apiCache.Close(); err != nil {
			slog.Warn("failed to close cache", "error", err)
		}
	}
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
