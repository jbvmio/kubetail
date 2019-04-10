package cmd

import (
	"regexp"
)

// RegexType defines a Regex Type.
type RegexType int

// Regex Types:
const (
	Black RegexType = 1
	White RegexType = 2
)

// RegexMaker here.
type RegexMaker interface {
	GetRegex() *regexp.Regexp
}

// RegexList implements RegexMaker.
type RegexList struct {
	Type RegexType
	List []string
}

// GetRegex takes a list of strings and returns a regular expression.
func (r *RegexList) GetRegex() *regexp.Regexp {
	var regexString string
	if len(r.List) < 1 {
		return nil
	}
	switch {
	case len(r.List) > 1:
		first := r.List[0]
		rest := r.List[1:]
		regexString += first
		for _, r := range rest {
			regexString += `|` + r
		}
	case len(r.List) == 1:
		regexString = r.List[0]
	}
	return regexp.MustCompile(regexString)
}
