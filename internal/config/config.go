package config

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/joho/godotenv"

	"pickcart/internal/domain"
)

const (
	LEDPin        = 18
	LEDFreqHz     = 800000
	LEDDMA        = 10
	LEDBrightness = 255
	LEDInvert     = false
	LEDChannel    = 0
	LEDCount      = 48
)

type AppConfig struct {
	OdooAPIKey  string
	OdooBaseURL string
	OdooDB      string
	CartNumber  int
	ProfileName string
}

var cartProfiles = map[string]domain.CartProfile{
	"standard": {
		Name:         "standard",
		DisplayName:  "Standard Cart",
		MaxBatchSize: 100,
		RowConfigs: []domain.RowConfig{
			{Cols: 28},
			{Cols: 28},
			{Cols: 14},
			{Cols: 14},
			{Cols: 8},
			{Cols: 8},
		},
	},
	"small_cart": {
		Name:         "small_cart",
		DisplayName:  "Small Cart",
		MaxBatchSize: 126,
		RowConfigs: []domain.RowConfig{
			{Cols: 28},
			{Cols: 28},
			{Cols: 28},
			{Cols: 14},
			{Cols: 14},
			{Cols: 14},
		},
	},
	"large_cart": {
		Name:         "large_cart",
		DisplayName:  "Large Cart",
		MaxBatchSize: 48,
		RowConfigs: []domain.RowConfig{
			{Cols: 8},
			{Cols: 8},
			{Cols: 8},
			{Cols: 8},
			{Cols: 8},
			{Cols: 8},
		},
	},
}

func Load(cartNumber int, profileName string) (*AppConfig, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	profile, err := GetCartProfile(profileName)
	if err != nil {
		return nil, err
	}

	return &AppConfig{
		OdooAPIKey:  strings.TrimSpace(os.Getenv("ODOO_API_KEY")),
		OdooBaseURL: strings.TrimSpace(os.Getenv("ODOO_BASE_URL")),
		OdooDB:      strings.TrimSpace(os.Getenv("ODOO_DATABASE")),
		CartNumber:  cartNumber,
		ProfileName: profile.Name,
	}, nil
}

func GetCartProfile(name string) (domain.CartProfile, error) {
	resolvedName := strings.TrimSpace(name)
	if resolvedName == "" {
		resolvedName = "standard"
	}

	profile, ok := cartProfiles[resolvedName]
	if !ok {
		return domain.CartProfile{}, fmt.Errorf("unknown cart profile %q", resolvedName)
	}

	copy := profile
	copy.RowConfigs = append([]domain.RowConfig(nil), profile.RowConfigs...)
	if profile.LEDColumnIndex != nil {
		idx := *profile.LEDColumnIndex
		copy.LEDColumnIndex = &idx
	}

	return copy, nil
}

func AvailableProfiles() []string {
	profiles := make([]string, 0, len(cartProfiles))
	for name := range cartProfiles {
		profiles = append(profiles, name)
	}
	sort.Strings(profiles)
	return profiles
}

func CartCapacity(p domain.CartProfile) int {
	if p.MaxBatchSize != 0 {
		return p.MaxBatchSize
	}
	return TotalCells(p)
}

func TotalCells(p domain.CartProfile) int {
	total := 0
	for _, row := range p.RowConfigs {
		total += row.Cols
	}
	return total
}
