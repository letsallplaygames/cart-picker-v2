package ui

import (
	"strings"
	"unicode"
)

func normalizedSearchQueries(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	upper := strings.ToUpper(raw)
	if strings.HasPrefix(upper, "1Z") {
		return dedupeQueries([]string{strings.ToLower(raw)})
	}

	digits := digitOnly(raw)
	longBarcode := isLongShippingLabelBarcode(raw)
	queries := make([]string, 0, 4)

	switch len(digits) {
	case 32:
		queries = appendFedExCandidates(queries, digits, 16, 12)
		if len(digits) >= 12 {
			queries = append(queries, digits[len(digits)-12:])
		}
	case 34:
		if strings.HasPrefix(digits, "420") && len(raw) > 8 {
			if suffix := strings.TrimSpace(raw[8:]); suffix != "" {
				queries = append(queries, strings.ToLower(suffix))
			}
		} else {
			queries = appendFedExCandidates(queries, digits, 16, 12)
			if len(digits) >= 12 {
				queries = append(queries, digits[len(digits)-12:])
			}
		}
	default:
		if longBarcode && len(raw) > 8 {
			if suffix := strings.TrimSpace(raw[8:]); suffix != "" {
				queries = append(queries, strings.ToLower(suffix))
			}
		}
		if longBarcode && len(digits) >= 34 {
			queries = appendFedExCandidates(queries, digits, 16, 12)
			queries = append(queries, digits[len(digits)-12:])
		}
	}

	if !longBarcode {
		queries = append(queries, strings.ToLower(raw))
	}

	return dedupeQueries(queries)
}

func appendFedExCandidates(queries []string, digits string, start, length int) []string {
	if start < 0 || length <= 0 || start+length > len(digits) {
		return queries
	}
	return append(queries, digits[start:start+length])
}

func dedupeQueries(queries []string) []string {
	seen := make(map[string]struct{}, len(queries))
	unique := make([]string, 0, len(queries))
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		if _, ok := seen[query]; ok {
			continue
		}
		seen[query] = struct{}{}
		unique = append(unique, query)
	}
	return unique
}

func looksLikeBarcodeScan(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if strings.HasPrefix(strings.ToUpper(raw), "1Z") {
		return true
	}
	if strings.Contains(raw, " ") {
		return false
	}
	if len(digitOnly(raw)) >= 20 {
		return true
	}
	return len(raw) >= 12 && isAlphanumeric(raw)
}

func isLongShippingLabelBarcode(raw string) bool {
	return len(digitOnly(raw)) >= 28
}

func isAlphanumeric(raw string) bool {
	for _, r := range raw {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func digitOnly(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func trackingMatches(stored, query string) bool {
	stored = normalizeTracking(stored)
	query = normalizeTracking(query)
	if stored == "" || query == "" {
		return false
	}
	if len(query) < 8 {
		return stored == query
	}
	if strings.Contains(stored, query) {
		return true
	}
	if len(stored) >= 8 && strings.Contains(query, stored) {
		return true
	}
	return false
}

func normalizeTracking(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
