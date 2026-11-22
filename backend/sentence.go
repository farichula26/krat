package main

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// minSentence avoids sending tiny fragments ("Sí.") to TTS on their own; they merge with the next one.
const minSentence = 20

// splitter cuts a streamed reply into sentences so TTS can start before the reply ends.
type splitter struct{ buf string }

func (s *splitter) Push(delta string) (out []string) {
	s.buf += delta
	rs := []rune(s.buf)
	start := 0
	for i := 0; i < len(rs)-1; i++ {
		if !strings.ContainsRune(".!?…\n", rs[i]) || !unicode.IsSpace(rs[i+1]) {
			continue
		}
		if seg := strings.TrimSpace(string(rs[start : i+1])); utf8.RuneCountInString(seg) >= minSentence {
			out = append(out, seg)
			start = i + 1
		}
	}
	s.buf = string(rs[start:])
	return out
}

func (s *splitter) Flush() []string {
	seg := strings.TrimSpace(s.buf)
	s.buf = ""
	if seg == "" {
		return nil
	}
	return []string{seg}
}
