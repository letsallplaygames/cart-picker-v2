package boxing

import (
	"fmt"
	"sort"
	"strings"

	"pickcart/internal/domain"
)

const cubicInchesPerCubicFoot = 1728.0

var Boxes = []domain.Box{
	newBox("UPS-Small", 9, 6, 3),
	newBox("UPS-Medium", 12.5, 10, 3),
	newBox("Two-Games", 11, 4.375, 7.25),
	newBox("Small", 13, 9, 4),
	newBox("Short-Medium", 13, 11, 6),
	newBox("Medium", 16, 12, 6),
	newBox("Tall-Medium", 13, 13, 13),
	newBox("Short-Large", 18, 14, 8),
	newBox("Tall-Large", 20, 12, 12),
	newBox("Long-Large", 24, 8, 8),
	newBox("Large-14", 22, 14, 14),
	newBox("Large-15", 26.875, 14.625, 15.625),
}

func FindSmallestBox(items []domain.Item) string {
	filtered := make([]domain.Item, 0, len(items))
	missingVolumes := make(map[string]struct{})
	totalVolume := 0.0

	for _, item := range items {
		if item.Quantity <= 0 {
			continue
		}

		filtered = append(filtered, item)
		if item.Volume <= 0 {
			missingVolumes[missingVolumeLabel(item)] = struct{}{}
			continue
		}

		totalVolume += item.Volume * float64(item.Quantity)
	}

	if len(filtered) == 0 {
		return ""
	}

	if len(missingVolumes) > 0 {
		labels := make([]string, 0, len(missingVolumes))
		for label := range missingVolumes {
			labels = append(labels, label)
		}
		sort.Strings(labels)
		return fmt.Sprintf("Oversize (Missing volume: %s)", strings.Join(labels, ","))
	}

	for _, box := range boxesByAscendingVolume() {
		if totalVolume <= box.Volume {
			return box.Name
		}
	}

	return "Oversize"
}

func newBox(name string, length, width, height float64) domain.Box {
	return domain.Box{
		Name:   name,
		Length: length,
		Width:  width,
		Height: height,
		Volume: (length * width * height) / cubicInchesPerCubicFoot,
	}
}

func boxesByAscendingVolume() []domain.Box {
	boxes := append([]domain.Box(nil), Boxes...)
	sort.SliceStable(boxes, func(i, j int) bool {
		if boxes[i].Volume == boxes[j].Volume {
			return boxes[i].Name < boxes[j].Name
		}
		return boxes[i].Volume < boxes[j].Volume
	})
	return boxes
}

func missingVolumeLabel(item domain.Item) string {
	if sku := strings.TrimSpace(item.SKU); sku != "" {
		return sku
	}
	if name := strings.TrimSpace(item.Name); name != "" {
		return name
	}
	return "UNKNOWN"
}
