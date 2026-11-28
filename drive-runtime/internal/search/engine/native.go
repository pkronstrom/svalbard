package engine

import (
	"context"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

// NativeZIMGroups lists the registered native sources in Kiwix-compatible
// language groups. Pre-migration databases have no native_language column.
func (e *Engine) NativeZIMGroups(ctx context.Context, scope string, excluded ...string) ([][]string, error) {
	if scope != "" && (!strings.EqualFold(filepath.Ext(scope), ".zim") || filepath.Base(scope) != scope) {
		return nil, nil
	}
	languages, err := e.nativeZIMLanguages(ctx, scope)
	if err != nil {
		return nil, err
	}
	for _, filename := range excluded {
		delete(languages, strings.TrimSuffix(filename, filepath.Ext(filename)))
	}
	var groups [][]string
	byLanguage := map[string]int{}
	for _, book := range slices.Sorted(maps.Keys(languages)) {
		lang := languages[book]
		if lang == "und" || strings.Contains(lang, ",") {
			// A missing or multilingual language may have a different
			// Xapian stemmer. Keep it in its own request.
			groups = append(groups, []string{book})
			continue
		}
		i, seen := byLanguage[lang]
		if !seen {
			i = len(groups)
			byLanguage[lang] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], book)
	}
	return groups, nil
}

// NativeZIMLanguages maps Kiwix book names to their archive language metadata.
func (e *Engine) NativeZIMLanguages(ctx context.Context) (map[string]string, error) {
	return e.nativeZIMLanguages(ctx, "")
}

func (e *Engine) nativeZIMLanguages(ctx context.Context, scope string) (map[string]string, error) {
	fields, err := e.db.QueryContext(ctx, "SELECT 1 FROM pragma_table_info('sources') WHERE name='native_language'")
	if err != nil {
		return nil, err
	}
	present := fields.Next()
	fieldErr := fields.Err()
	fields.Close()
	if !present || fieldErr != nil {
		return nil, fieldErr
	}
	rows, err := e.db.QueryContext(ctx, `SELECT filename, native_language FROM sources
		WHERE native_language <> '' AND (? = '' OR filename = ?)`, scope, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	languages := map[string]string{}
	for rows.Next() {
		var filename, lang string
		if err := rows.Scan(&filename, &lang); err != nil {
			return nil, err
		}
		languages[strings.TrimSuffix(filename, filepath.Ext(filename))] = lang
	}
	return languages, rows.Err()
}
