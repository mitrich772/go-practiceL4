// Package grep содержит чистую логику matching'а строки против паттерна.
// Логика не зависит от транспорта — её удобно покрывать unit-тестами и
// переиспользовать как из сервера, так и из локальных бенчмарков.
package grep

import (
	"fmt"
	"regexp"
	"strings"

	"mygrep/internal/protocol"
)

// Matcher — скомпилированный паттерн с заранее выставленными опциями.
type Matcher struct {
	flags protocol.GrepFlags
	re    *regexp.Regexp

	// Предвычисленный паттерн с учётом IgnoreCase для FixedString-режима.
	fixedNeedle string
}

// NewMatcher разбирает Pattern с учётом флагов и возвращает готовый Matcher.
// Возвращает ошибку только если паттерн — некорректный regexp.
func NewMatcher(flags protocol.GrepFlags) (*Matcher, error) {
	m := &Matcher{flags: flags}

	if flags.FixedString {
		needle := flags.Pattern
		if flags.IgnoreCase {
			needle = strings.ToLower(needle)
		}
		m.fixedNeedle = needle
		return m, nil
	}

	pattern := flags.Pattern
	if flags.IgnoreCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile pattern: %w", err)
	}
	m.re = re
	return m, nil
}

// Match возвращает true, если строка соответствует паттерну (с учётом -v).
func (m *Matcher) Match(line string) bool {
	var hit bool
	if m.flags.FixedString {
		hay := line
		if m.flags.IgnoreCase {
			hay = strings.ToLower(hay)
		}
		hit = strings.Contains(hay, m.fixedNeedle)
	} else {
		hit = m.re.MatchString(line)
	}
	if m.flags.InvertMatch {
		hit = !hit
	}
	return hit
}
