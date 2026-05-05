package sensitivewords

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatcherDetectsStrictAndCompactMatches(t *testing.T) {
	entries := []Entry{
		{Word: "习近平", Category: "政治", Source: "政治类型.txt"},
		{Word: "法轮功", Category: "暴恐", Source: "暴恐词库.txt"},
		{Word: "000.2011wyt.com", Category: "网址", Source: "非法网址.txt"},
	}

	matcher, err := NewMatcher(entries, Options{})
	if err != nil {
		t.Fatalf("NewMatcher() error = %v", err)
	}

	result := matcher.Detect("测试 习-近-平 和法.轮.功，还有 000.2011wyt.com")
	if !result.Violates {
		t.Fatalf("Detect().Violates = false, want true")
	}
	if len(result.Matches) != 3 {
		t.Fatalf("Detect() match count = %d, want 3", len(result.Matches))
	}

	if result.Matches[0].Mode != "compact" {
		t.Fatalf("first match mode = %q, want compact", result.Matches[0].Mode)
	}
	if result.Matches[1].Mode != "compact" {
		t.Fatalf("second match mode = %q, want compact", result.Matches[1].Mode)
	}
	if result.Matches[2].Mode != "strict" {
		t.Fatalf("third match mode = %q, want strict", result.Matches[2].Mode)
	}
}

func TestMatcherSkipsSingleRuneByDefault(t *testing.T) {
	entries := []Entry{
		{Word: "党", Category: "政治", Source: "政治类型.txt"},
	}

	matcher, err := NewMatcher(entries, Options{})
	if err == nil {
		t.Fatalf("NewMatcher() error = nil, want filtering error because all patterns are single rune")
	}

	matcher, err = NewMatcher(entries, Options{IncludeSingleRune: true, MinWordRunes: 1, MinCompactWordRunes: 2})
	if err != nil {
		t.Fatalf("NewMatcher() with single rune enabled error = %v", err)
	}
	if !matcher.Contains("这个党字会被命中") {
		t.Fatalf("Contains() = false, want true")
	}
}

func TestNewEmptyMatcherSupportsDynamicAddAndCount(t *testing.T) {
	matcher := NewEmptyMatcher(Options{})

	if matcher.Count() != 0 {
		t.Fatalf("Count() = %d, want 0", matcher.Count())
	}
	if matcher.Contains("习近平") {
		t.Fatalf("Contains() = true, want false")
	}

	if !matcher.AddWord("习近平") {
		t.Fatalf("AddWord() = false, want true")
	}
	if matcher.AddWord("习近平") {
		t.Fatalf("duplicate AddWord() = true, want false")
	}
	if matcher.AddWords([]string{"法轮功", "法轮功", "党"}) != 1 {
		t.Fatalf("AddWords() did not ignore duplicates/filtered words as expected")
	}

	if matcher.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", matcher.Count())
	}
	if !matcher.Contains("这里有习-近-平") {
		t.Fatalf("Contains() = false, want true for compact match")
	}

	stats := matcher.Stats()
	if stats.WordCount != 2 {
		t.Fatalf("Stats().WordCount = %d, want 2", stats.WordCount)
	}
	if stats.EntryCount != 2 {
		t.Fatalf("Stats().EntryCount = %d, want 2", stats.EntryCount)
	}
}

func TestLoadEntriesFromDirAndAggregateSources(t *testing.T) {
	dir := t.TempDir()

	fileA := filepath.Join(dir, "政治类型.txt")
	fileB := filepath.Join(dir, "补充词库.txt")

	if err := os.WriteFile(fileA, []byte("习近平\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(fileA) error = %v", err)
	}
	if err := os.WriteFile(fileB, []byte("习近平\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(fileB) error = %v", err)
	}

	matcher, err := NewMatcherFromDir(dir, Options{})
	if err != nil {
		t.Fatalf("NewMatcherFromDir() error = %v", err)
	}

	result := matcher.Detect("习近平")
	if len(result.Matches) != 1 {
		t.Fatalf("Detect() match count = %d, want 1", len(result.Matches))
	}
	if len(result.Matches[0].Categories) != 2 {
		t.Fatalf("categories count = %d, want 2", len(result.Matches[0].Categories))
	}
	if len(result.Matches[0].Sources) != 2 {
		t.Fatalf("sources count = %d, want 2", len(result.Matches[0].Sources))
	}
}

func TestMatcherCanLoadFromFileAndDirAfterCreation(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "政治类型.txt")
	fileB := filepath.Join(dir, "暴恐词库.txt")

	if err := os.WriteFile(fileA, []byte("习近平\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(fileA) error = %v", err)
	}
	if err := os.WriteFile(fileB, []byte("法轮功\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(fileB) error = %v", err)
	}

	matcher := NewEmptyMatcher(Options{})

	added, err := matcher.LoadFromFile(fileA)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	if added != 1 {
		t.Fatalf("LoadFromFile() added = %d, want 1", added)
	}

	added, err = matcher.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	if added != 1 {
		t.Fatalf("LoadFromDir() added = %d, want 1 because one word already existed", added)
	}

	if matcher.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", matcher.Count())
	}
	if !matcher.Contains("法.轮.功") {
		t.Fatalf("Contains() = false, want true")
	}
}
