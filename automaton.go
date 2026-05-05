package sensitivewords

type automaton struct {
	nodes    []node
	patterns []pattern
}

type node struct {
	next    map[rune]int
	fail    int
	outputs []int
}

type pattern struct {
	key        string
	length     int
	categories []string
	sources    []string
}

type rawMatch struct {
	patternID int
	startNorm int
	endNorm   int
}

func newAutomaton(patterns []pattern) *automaton {
	a := &automaton{
		nodes:    []node{{next: make(map[rune]int)}},
		patterns: patterns,
	}

	for idx, pattern := range patterns {
		a.add(pattern.key, idx)
	}
	a.build()
	return a
}

func (a *automaton) add(word string, patternID int) {
	state := 0
	for _, r := range word {
		next, ok := a.nodes[state].next[r]
		if !ok {
			next = len(a.nodes)
			a.nodes = append(a.nodes, node{next: make(map[rune]int)})
			a.nodes[state].next[r] = next
		}
		state = next
	}
	a.nodes[state].outputs = append(a.nodes[state].outputs, patternID)
}

func (a *automaton) build() {
	queue := make([]int, 0, len(a.nodes))

	for _, next := range a.nodes[0].next {
		a.nodes[next].fail = 0
		queue = append(queue, next)
	}

	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]

		for r, next := range a.nodes[state].next {
			queue = append(queue, next)

			fail := a.nodes[state].fail
			for fail != 0 {
				if _, ok := a.nodes[fail].next[r]; ok {
					break
				}
				fail = a.nodes[fail].fail
			}

			if fallback, ok := a.nodes[fail].next[r]; ok && fallback != next {
				a.nodes[next].fail = fallback
			}

			failOutputs := a.nodes[a.nodes[next].fail].outputs
			if len(failOutputs) > 0 {
				a.nodes[next].outputs = append(a.nodes[next].outputs, failOutputs...)
			}
		}
	}
}

func (a *automaton) findAll(text normalizedText, limit int) []rawMatch {
	if len(text.text) == 0 {
		return nil
	}

	matches := make([]rawMatch, 0, 4)
	state := 0

	for idx, r := range text.text {
		for state != 0 {
			if _, ok := a.nodes[state].next[r]; ok {
				break
			}
			state = a.nodes[state].fail
		}

		if next, ok := a.nodes[state].next[r]; ok {
			state = next
		}

		for _, patternID := range a.nodes[state].outputs {
			pattern := a.patterns[patternID]
			match := rawMatch{
				patternID: patternID,
				startNorm: idx - pattern.length + 1,
				endNorm:   idx + 1,
			}
			matches = append(matches, match)
			if limit > 0 && len(matches) >= limit {
				return matches
			}
		}
	}

	return matches
}

func (a *automaton) contains(text normalizedText) bool {
	if len(text.text) == 0 {
		return false
	}

	state := 0
	for _, r := range text.text {
		for state != 0 {
			if _, ok := a.nodes[state].next[r]; ok {
				break
			}
			state = a.nodes[state].fail
		}

		if next, ok := a.nodes[state].next[r]; ok {
			state = next
		}

		if len(a.nodes[state].outputs) > 0 {
			return true
		}
	}

	return false
}
