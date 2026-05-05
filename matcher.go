package sensitivewords

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Options controls lexicon filtering and matching strategy.
type Options struct {
	MinWordRunes        int
	MinCompactWordRunes int
	IncludeSingleRune   bool
}

// Matcher detects sensitive words against a loaded lexicon.
type Matcher struct {
	mu       sync.RWMutex
	options  Options
	entries  []Entry
	entrySet map[string]struct{}
	wordSet  map[string]struct{}
	strict   *automaton
	compact  *automaton
}

// Match is one detected sensitive term in the input text.
type Match struct {
	Keyword    string
	Matched    string
	Categories []string
	Sources    []string
	StartRune  int
	EndRune    int
	Mode       string
}

// Result is the moderation decision for one text input.
type Result struct {
	Violates bool
	Matches  []Match
}

// Stats describes the current in-memory lexicon state.
type Stats struct {
	EntryCount          int
	WordCount           int
	StrictPatternCount  int
	CompactPatternCount int
}

// NewEmptyMatcher creates an empty matcher that can be filled from local files
// or dynamic AddWord/AddWords calls later.
func NewEmptyMatcher(options Options) *Matcher {
	options = options.withDefaults()
	return &Matcher{
		options:  options,
		entrySet: make(map[string]struct{}),
		wordSet:  make(map[string]struct{}),
		strict:   newAutomaton(nil),
		compact:  newAutomaton(nil),
	}
}

// NewMatcherFromDir builds a matcher from all .txt files in a lexicon directory.
func NewMatcherFromDir(dir string, options Options) (*Matcher, error) {
	entries, err := LoadEntriesFromDir(dir)
	if err != nil {
		return nil, err
	}
	return NewMatcher(entries, options)
}

// NewMatcher builds a matcher from raw entries.
func NewMatcher(entries []Entry, options Options) (*Matcher, error) {
	matcher := NewEmptyMatcher(options)
	added := matcher.AddEntries(entries)
	if added == 0 {
		return nil, fmt.Errorf("no patterns left after normalization and filtering")
	}
	return matcher, nil
}

// Contains returns true as soon as any sensitive word is found.
func (m *Matcher) Contains(text string) bool {
	return m.Detect(text).Violates
}

// LoadFromDir loads all .txt files from a local directory into memory.
func (m *Matcher) LoadFromDir(dir string) (int, error) {
	entries, err := LoadEntriesFromDir(dir)
	if err != nil {
		return 0, err
	}
	return m.AddEntries(entries), nil
}

// LoadFromFile loads one local .txt file into memory.
func (m *Matcher) LoadFromFile(path string) (int, error) {
	entries, err := LoadEntriesFromFile(path)
	if err != nil {
		return 0, err
	}
	return m.AddEntries(entries), nil
}

// AddWord inserts one sensitive word into the current in-memory lexicon.
// It returns false when the word is empty, filtered by current options, or
// already present.
func (m *Matcher) AddWord(word string) bool {
	return m.AddEntry(Entry{Word: word})
}

// AddWords inserts multiple sensitive words and returns the count of newly
// accepted words.
func (m *Matcher) AddWords(words []string) int {
	if len(words) == 0 {
		return 0
	}

	entries := make([]Entry, 0, len(words))
	for _, word := range words {
		entries = append(entries, Entry{Word: word})
	}
	return m.AddEntries(entries)
}

// AddEntry inserts one sensitive entry, preserving category and source metadata.
func (m *Matcher) AddEntry(entry Entry) bool {
	return m.AddEntries([]Entry{entry}) == 1
}

// AddEntries inserts multiple sensitive entries and rebuilds the automata once.
// Duplicate or filtered-out entries are ignored.
func (m *Matcher) AddEntries(entries []Entry) int {
	prepared := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		cleaned, ok := sanitizeEntry(entry, m.options)
		if !ok {
			continue
		}
		prepared = append(prepared, cleaned)
	}
	if len(prepared) == 0 {
		return 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	added := 0
	for _, entry := range prepared {
		key := entryKey(entry)
		if _, ok := m.entrySet[key]; ok {
			continue
		}

		m.entrySet[key] = struct{}{}
		m.entries = append(m.entries, entry)

		if wordKey := wordKey(entry.Word); wordKey != "" {
			m.wordSet[wordKey] = struct{}{}
		}
		added++
	}

	if added > 0 {
		m.rebuildLocked()
	}

	return added
}

// Count returns the number of unique in-memory sensitive words after
// normalization and de-duplication.
func (m *Matcher) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.wordSet)
}

// Stats returns the current lexicon counters.
func (m *Matcher) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Stats{
		EntryCount:          len(m.entries),
		WordCount:           len(m.wordSet),
		StrictPatternCount:  len(m.strict.patterns),
		CompactPatternCount: len(m.compact.patterns),
	}
}

// Detect returns all detected matches after strict and compact scanning.
func (m *Matcher) Detect(text string) Result {
	m.mu.RLock()
	strict := m.strict
	compact := m.compact
	m.mu.RUnlock()

	strictText := normalizeInput(text, normalizeStrict)
	compactText := normalizeInput(text, normalizeCompact)

	merged := make(map[string]Match)

	for _, raw := range strict.findAll(strictText, 0) {
		pattern := strict.patterns[raw.patternID]
		match := toMatch(pattern, strictText, raw, "strict")
		merged[matchKey(match)] = match
	}

	for _, raw := range compact.findAll(compactText, 0) {
		pattern := compact.patterns[raw.patternID]
		match := toMatch(pattern, compactText, raw, "compact")
		key := matchKey(match)

		existing, ok := merged[key]
		if !ok {
			merged[key] = match
			continue
		}
		if existing.Mode == "strict" {
			continue
		}
		merged[key] = match
	}

	matches := make([]Match, 0, len(merged))
	for _, match := range merged {
		matches = append(matches, match)
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].StartRune != matches[j].StartRune {
			return matches[i].StartRune < matches[j].StartRune
		}
		if matches[i].EndRune != matches[j].EndRune {
			return matches[i].EndRune > matches[j].EndRune
		}
		return matchPriority(matches[i]) > matchPriority(matches[j])
	})

	matches = pruneOverlaps(matches)

	return Result{
		Violates: len(matches) > 0,
		Matches:  matches,
	}
}

func (o Options) withDefaults() Options {
	if o.MinWordRunes == 0 {
		o.MinWordRunes = 2
	}
	if o.MinCompactWordRunes == 0 {
		o.MinCompactWordRunes = 3
	}
	return o
}

func (m *Matcher) rebuildLocked() {
	strictPatterns := buildPatterns(m.entries, m.options, normalizeStrict)
	compactPatterns := buildPatterns(m.entries, m.options, normalizeCompact)
	m.strict = newAutomaton(strictPatterns)
	m.compact = newAutomaton(compactPatterns)
}

func buildPatterns(entries []Entry, options Options, mode normalizeMode) []pattern {
	grouped := make(map[string]*pattern)

	for _, entry := range entries {
		normalized := normalizeKeyword(entry.Word, mode)
		if normalized == "" {
			continue
		}

		length := runeLen(normalized)
		if !options.IncludeSingleRune && length == 1 {
			continue
		}

		minLen := options.MinWordRunes
		if mode == normalizeCompact {
			minLen = options.MinCompactWordRunes
		}
		if length < minLen {
			continue
		}

		existing, ok := grouped[normalized]
		if !ok {
			grouped[normalized] = &pattern{
				key:        normalized,
				length:     length,
				categories: []string{entry.Category},
				sources:    []string{entry.Source},
			}
			continue
		}

		existing.categories = append(existing.categories, entry.Category)
		existing.sources = append(existing.sources, entry.Source)
	}

	patterns := make([]pattern, 0, len(grouped))
	for _, item := range grouped {
		item.categories = uniqueStrings(item.categories)
		item.sources = uniqueStrings(item.sources)
		patterns = append(patterns, *item)
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].key < patterns[j].key
	})

	return patterns
}

func toMatch(pattern pattern, text normalizedText, raw rawMatch, mode string) Match {
	startOriginal, endOriginal, matched := sliceOriginalRunes(text, raw.startNorm, raw.endNorm)
	return Match{
		Keyword:    pattern.key,
		Matched:    matched,
		Categories: append([]string(nil), pattern.categories...),
		Sources:    append([]string(nil), pattern.sources...),
		StartRune:  startOriginal,
		EndRune:    endOriginal,
		Mode:       mode,
	}
}

func matchKey(match Match) string {
	return fmt.Sprintf("%d:%d:%s", match.StartRune, match.EndRune, match.Keyword)
}

func pruneOverlaps(matches []Match) []Match {
	if len(matches) < 2 {
		return matches
	}

	kept := make([]Match, 0, len(matches))
	for _, candidate := range matches {
		covered := false
		for _, existing := range kept {
			if !overlaps(existing, candidate) {
				continue
			}
			covered = true
			break
		}
		if !covered {
			kept = append(kept, candidate)
		}
	}

	return kept
}

func overlaps(a, b Match) bool {
	return a.StartRune < b.EndRune && b.StartRune < a.EndRune
}

func matchPriority(match Match) int {
	span := match.EndRune - match.StartRune
	priority := span * 100
	if match.Mode == "strict" {
		priority += 10
	}
	priority += runeLen(match.Keyword)
	return priority
}

func sanitizeEntry(entry Entry, options Options) (Entry, bool) {
	entry.Word = strings.TrimSpace(strings.TrimRight(entry.Word, "\r"))
	entry.Category = strings.TrimSpace(entry.Category)
	entry.Source = strings.TrimSpace(entry.Source)

	if entry.Word == "" {
		return Entry{}, false
	}
	if !entryAccepted(entry.Word, options.MinWordRunes, options.IncludeSingleRune) &&
		!entryAcceptedForMode(entry.Word, options.MinCompactWordRunes, options.IncludeSingleRune, normalizeCompact) {
		return Entry{}, false
	}
	return entry, true
}

func entryAccepted(word string, minRunes int, includeSingleRune bool) bool {
	return normalizedWordAccepted(normalizeKeyword(word, normalizeStrict), minRunes, includeSingleRune)
}

func entryAcceptedForMode(word string, minRunes int, includeSingleRune bool, mode normalizeMode) bool {
	return normalizedWordAccepted(normalizeKeyword(word, mode), minRunes, includeSingleRune)
}

func normalizedWordAccepted(word string, minRunes int, includeSingleRune bool) bool {
	if word == "" {
		return false
	}

	length := runeLen(word)
	if !includeSingleRune && length == 1 {
		return false
	}

	return length >= minRunes
}

func entryKey(entry Entry) string {
	return normalizeKeyword(entry.Word, normalizeStrict) + "\x00" + entry.Category + "\x00" + entry.Source
}

func wordKey(word string) string {
	return normalizeKeyword(word, normalizeStrict)
}
