package editorbuffer

import (
	"errors"
	"fmt"
	"regexp/syntax"
	"strings"
	"unicode/utf16"
)

// PatternError is a regular expression of the Find window that cannot be
// used, in plain words, with the position of the problem in the pattern
// (UTF-16 units, as the page counts them). The Find window's syntax guide
// lists only what this engine supports; the rest is refused here with a
// message that names it, for every document.
type PatternError struct {
	Message string `json:"error"`
	Offset  int64  `json:"at"`
}

func (e *PatternError) Error() string {
	return fmt.Sprintf("Invalid regular expression at character %d: %s", e.Offset+1, e.Message)
}

// CheckPattern says whether the Find what text of a regular-expression
// search can be used.
func CheckPattern(query string) *PatternError {
	if _, err := syntax.Parse(query, syntax.Perl); err != nil {
		return patternError(query, err)
	}
	return nil
}

func unitsBefore(s string, at int) int64 {
	if at < 0 || at > len(s) {
		at = 0
	}
	return int64(len(utf16.Encode([]rune(s[:at]))))
}

func patternError(query string, err error) *PatternError {
	var e *syntax.Error
	if !errors.As(err, &e) {
		return &PatternError{Message: err.Error()}
	}
	at := strings.Index(query, e.Expr)
	switch e.Code {
	case syntax.ErrMissingParen:
		at = unmatched(query, '(')
		return &PatternError{"A ( has no closing ).", unitsBefore(query, at)}
	case syntax.ErrUnexpectedParen:
		at = unmatched(query, ')')
		return &PatternError{"A ) has no opening (.", unitsBefore(query, at)}
	case syntax.ErrMissingBracket:
		return &PatternError{"A [ has no closing ].", unitsBefore(query, at)}
	case syntax.ErrInvalidEscape:
		message := "Unknown escape " + e.Expr + ": write \\\\ for a backslash."
		switch {
		case len(e.Expr) == 2 && e.Expr[1] >= '1' && e.Expr[1] <= '9':
			message = "Backreferences such as " + e.Expr + " are not supported in Find what (use $1 in Replace with)."
		case strings.HasPrefix(e.Expr, `\u`):
			message = `\uNNNN is not supported in a regular expression: use \x{NNNN}, or Extended mode.`
		}
		return &PatternError{message, unitsBefore(query, at)}
	case syntax.ErrInvalidPerlOp:
		message := "This kind of group, " + e.Expr + ", is not supported."
		for _, look := range []string{"(?=", "(?!", "(?<=", "(?<!"} {
			if strings.HasPrefix(e.Expr, look) {
				message = "Lookahead and lookbehind ((?=, (?!, (?<=, (?<!) are not supported."
			}
		}
		if strings.HasPrefix(e.Expr, "(?>") {
			message = "Atomic groups (?>…) are not supported."
		}
		return &PatternError{message, unitsBefore(query, at)}
	case syntax.ErrMissingRepeatArgument:
		return &PatternError{"Nothing to repeat before " + e.Expr + ".", unitsBefore(query, at)}
	case syntax.ErrInvalidRepeatOp:
		message := "Two repeats in a row (" + e.Expr + ")."
		if strings.HasSuffix(e.Expr, "+") && len(e.Expr) > 1 {
			message = "Possessive repeats such as " + e.Expr + " are not supported."
		}
		return &PatternError{message, unitsBefore(query, at)}
	case syntax.ErrInvalidRepeatSize:
		return &PatternError{"The repeat count " + e.Expr + " is too large or out of order.", unitsBefore(query, at)}
	case syntax.ErrInvalidCharRange:
		return &PatternError{"The range " + e.Expr + " runs backwards.", unitsBefore(query, at)}
	case syntax.ErrInvalidCharClass:
		return &PatternError{"Unknown character class " + e.Expr + ".", unitsBefore(query, at)}
	case syntax.ErrTrailingBackslash:
		return &PatternError{"The pattern ends with a lone \\.", unitsBefore(query, len(query)-1)}
	case syntax.ErrInvalidNamedCapture:
		if strings.HasPrefix(e.Expr, "(?<=") || strings.HasPrefix(e.Expr, "(?<!") {
			return &PatternError{"Lookahead and lookbehind ((?=, (?!, (?<=, (?<!) are not supported.", unitsBefore(query, at)}
		}
		return &PatternError{"Invalid group name " + e.Expr + ".", unitsBefore(query, at)}
	case syntax.ErrNestingDepth, syntax.ErrLarge:
		return &PatternError{"The pattern is too complex.", 0}
	}
	return &PatternError{e.Code.String() + ": " + e.Expr, unitsBefore(query, at)}
}

// unmatched is the byte position of the first parenthesis of kind that has
// no partner, outside escapes and character classes.
func unmatched(query string, kind byte) int {
	var open []int
	class := false
	for i := 0; i < len(query); i++ {
		switch c := query[i]; {
		case c == '\\':
			i++
		case class:
			if c == ']' {
				class = false
			}
		case c == '[':
			class = true
			if i+1 < len(query) && query[i+1] == ']' {
				i++
			}
		case c == '(':
			open = append(open, i)
		case c == ')':
			if len(open) == 0 {
				if kind == ')' {
					return i
				}
				continue
			}
			open = open[:len(open)-1]
		}
	}
	if kind == '(' && len(open) > 0 {
		return open[len(open)-1]
	}
	return 0
}
