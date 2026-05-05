package sensitivewords

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry is one raw lexicon item loaded from a source file.
type Entry struct {
	Word     string
	Category string
	Source   string
}

// LoadEntriesFromDir loads all .txt lexicon files from a directory.
func LoadEntriesFromDir(dir string) ([]Entry, error) {
	entries := make([]Entry, 0, 1024)

	matches, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		return nil, fmt.Errorf("glob lexicon files: %w", err)
	}
	sort.Strings(matches)

	for _, path := range matches {
		fileEntries, err := loadEntriesFromFile(path)
		if err != nil {
			return nil, err
		}
		entries = append(entries, fileEntries...)
	}

	return entries, nil
}

// LoadEntriesFromFile loads one local .txt lexicon file.
func LoadEntriesFromFile(path string) ([]Entry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open lexicon file %q: %w", path, err)
	}
	defer file.Close()

	category := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	entries := make([]Entry, 0, 256)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimRight(scanner.Text(), "\r"))
		if line == "" {
			continue
		}
		entries = append(entries, Entry{
			Word:     line,
			Category: category,
			Source:   path,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan lexicon file %q: %w", path, err)
	}

	return entries, nil
}

func loadEntriesFromFile(path string) ([]Entry, error) {
	return LoadEntriesFromFile(path)
}
