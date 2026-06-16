package filter

import "strings"

type Filter struct {
	RunWords  []string `json:"run_words"`
	StopWords []string `json:"stop_words"`
}

func (f Filter) Matches(text string) bool {
	lower := strings.ToLower(text)
	for _, sw := range f.StopWords {
		if strings.Contains(lower, strings.ToLower(sw)) {
			return false
		}
	}
	if len(f.RunWords) == 0 {
		return true
	}
	for _, rw := range f.RunWords {
		if strings.Contains(lower, strings.ToLower(rw)) {
			return true
		}
	}
	return false
}
