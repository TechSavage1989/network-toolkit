package editorbuffer

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The Replace text of a regular-expression search is a template (the Find
// window's "Regular expression" search mode). It is expanded here for every
// document, page-held text too (ReplaceInText, find.go): the page has no
// second implementation. testdata/replace-template.json holds the cases.
//
//	$1 .. $99   a group; two digits name a group only when it exists, so with
//	            three groups "$12" is group 1 followed by "2"
//	$0 or $&    the whole match
//	${name}     a named (or numbered) group
//	$$          a dollar sign
//	\n \t       a line break, a tab (\r and \r\n are a line break too)
//	\\ \$       a backslash, a dollar sign
//	\xNN \uNNNN the character with that code
//
// A reference to a group that does not exist, and any other backslash, stay
// as written. \0 is refused: the editor cannot hold a NUL character.

type templatePart struct {
	text  string
	group int // -1 for text
}

// ErrTemplateNUL refuses a Replace text that would add a NUL character.
var ErrTemplateNUL = errors.New("The replacement would add a NUL character, which the Text Editor cannot hold.")

func parseTemplate(template string, names []string) ([]templatePart, error) {
	groups := len(names) - 1
	var parts []templatePart
	var text strings.Builder
	literal := func(s string) { text.WriteString(s) }
	group := func(n int) {
		if text.Len() > 0 {
			parts = append(parts, templatePart{text: text.String(), group: -1})
			text.Reset()
		}
		parts = append(parts, templatePart{group: n})
	}
	for i := 0; i < len(template); {
		c := template[i]
		switch c {
		case '$':
			if i+1 >= len(template) {
				literal("$")
				i++
				continue
			}
			next := template[i+1]
			switch {
			case next == '$':
				literal("$")
				i += 2
			case next == '&':
				group(0)
				i += 2
			case next >= '0' && next <= '9':
				n := int(next - '0')
				if i+2 < len(template) && template[i+2] >= '0' && template[i+2] <= '9' {
					if two := n*10 + int(template[i+2]-'0'); two >= 1 && two <= groups {
						group(two)
						i += 3
						continue
					}
				}
				if n <= groups {
					group(n)
				} else {
					literal(template[i : i+2])
				}
				i += 2
			case next == '{':
				end := strings.IndexByte(template[i+2:], '}')
				if end < 0 {
					literal("$")
					i++
					continue
				}
				name := template[i+2 : i+2+end]
				found := -1
				if n, err := strconv.Atoi(name); err == nil && n >= 0 && n <= groups && name == strconv.Itoa(n) {
					found = n
				} else if name != "" {
					for k := 1; k <= groups; k++ {
						if names[k] == name {
							found = k
							break
						}
					}
				}
				if found < 0 {
					literal(template[i : i+3+end])
				} else {
					group(found)
				}
				i += 3 + end
			default:
				literal("$")
				i++
			}
		case '\\':
			if i+1 >= len(template) {
				literal(`\`)
				i++
				continue
			}
			switch next := template[i+1]; next {
			case 'n':
				literal("\n")
				i += 2
			case 'r':
				literal("\n")
				i += 2
				if strings.HasPrefix(template[i:], `\n`) {
					i += 2
				}
			case 't':
				literal("\t")
				i += 2
			case '\\':
				literal(`\`)
				i += 2
			case '$':
				literal("$")
				i += 2
			case '0':
				return nil, ErrTemplateNUL
			case 'x', 'u':
				width := 2
				if next == 'u' {
					width = 4
				}
				if i+2+width <= len(template) {
					if v, err := strconv.ParseUint(template[i+2:i+2+width], 16, 32); err == nil {
						r := rune(v)
						switch {
						case r == 0:
							return nil, ErrTemplateNUL
						case r == '\r':
							r = '\n'
						case r >= 0xd800 && r < 0xe000:
							r = utf8.RuneError
						}
						literal(string(r))
						i += 2 + width
						continue
					}
				}
				literal(template[i : i+2])
				i += 2
			default:
				literal(template[i : i+2])
				i += 2
			}
		default:
			text.WriteByte(c)
			i++
		}
	}
	if text.Len() > 0 || len(parts) == 0 {
		parts = append(parts, templatePart{text: text.String(), group: -1})
	}
	return parts, nil
}

// expandTemplate expands parts for one match; groups holds the text of each
// group (nil when the group did not take part). It is the bounded form used
// for a single Replace; Replace All copies group text from the document.
func expandTemplate(parts []templatePart, groups []string) string {
	var out strings.Builder
	for _, p := range parts {
		if p.group < 0 {
			out.WriteString(p.text)
		} else if p.group < len(groups) {
			out.WriteString(groups[p.group])
		}
	}
	return out.String()
}
