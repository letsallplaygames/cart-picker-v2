package led

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"pickcart/internal/config"
)

const mappingFileName = "pick_shelf_light_positions.csv"

func loadMappingsForCart(cartNumber int, profileName string) (map[string]int, int, string, int, error) {
	profile, err := config.GetCartProfile(profileName)
	if err != nil {
		return nil, 0, "", 0, fmt.Errorf("resolve cart profile: %w", err)
	}

	columnIndex := cartNumber
	if profile.LEDColumnIndex != nil {
		columnIndex = *profile.LEDColumnIndex
	}
	if columnIndex < 1 {
		return nil, 0, "", columnIndex, fmt.Errorf("invalid LED mapping column index %d", columnIndex)
	}

	path, err := findMappingFile()
	if err != nil {
		return nil, 0, "", columnIndex, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, 0, path, columnIndex, fmt.Errorf("open LED mapping file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	mappings := map[string]int{}
	maxIndex := -1
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, path, columnIndex, fmt.Errorf("read LED mapping CSV: %w", err)
		}
		if len(row) <= columnIndex {
			continue
		}

		position := strings.ToUpper(strings.TrimSpace(row[0]))
		if position == "" {
			continue
		}

		rawIndex := strings.TrimSpace(row[columnIndex])
		if rawIndex == "" {
			continue
		}

		ledIndex, err := strconv.Atoi(rawIndex)
		if err != nil {
			continue
		}

		mappings[position] = ledIndex
		if ledIndex > maxIndex {
			maxIndex = ledIndex
		}
	}

	if len(mappings) == 0 {
		return nil, 0, path, columnIndex, fmt.Errorf("no LED mappings found in column %d", columnIndex)
	}

	return mappings, maxIndex + 1, path, columnIndex, nil
}

func findMappingFile() (string, error) {
	candidates := []string{}

	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		candidates = append(candidates, filepath.Join(cwd, mappingFileName))
	}

	if exePath, err := os.Executable(); err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, mappingFileName),
			filepath.Join(exeDir, "..", mappingFileName),
		)
	}

	seen := map[string]struct{}{}
	checked := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		clean := filepath.Clean(candidate)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		checked = append(checked, clean)
		if info, err := os.Stat(clean); err == nil && !info.IsDir() {
			return clean, nil
		}
	}

	return "", fmt.Errorf("LED mapping file %q not found; checked %s", mappingFileName, strings.Join(checked, ", "))
}
