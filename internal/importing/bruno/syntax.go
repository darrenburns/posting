// Package bruno imports Bruno's text .bru request and collection format.
package bruno

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/darrenburns/posting/internal/model"
)

const maxFileSize = 16 << 20

type block struct{ name, text string }
type document []block

// Bru text blocks end at a column-zero closing brace, even when their content
// contains JSON or JavaScript braces. This follows the upstream v2 grammar.
func parseDocument(data []byte) (document, error) {
	if len(data) > maxFileSize {
		return nil, fmt.Errorf("Bruno file exceeds 16 MiB")
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("Bruno file is not valid UTF-8")
	}
	s := strings.TrimPrefix(string(data), "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	var d document
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		p := strings.IndexByte(line, '{')
		if p < 1 {
			return nil, fmt.Errorf("line %d: expected a Bruno block opening", i+1)
		}
		name := strings.TrimSpace(line[:p])
		for _, c := range name {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == ':' || c == '-') {
				return nil, fmt.Errorf("line %d: invalid block name", i+1)
			}
		}
		start := i + 1
		i++
		// Triple-quoted dictionary values may themselves contain column-zero braces.
		triple := false
		textBlock := isText(name)
		for ; i < len(lines); i++ {
			if !triple && strings.HasPrefix(lines[i], "}") {
				if strings.TrimSpace(lines[i]) != "}" {
					return nil, fmt.Errorf("line %d: unexpected text after block", i+1)
				}
				break
			}
			if !textBlock && strings.Count(lines[i], "'''")%2 == 1 {
				triple = !triple
			}
		}
		if i == len(lines) {
			return nil, fmt.Errorf("unclosed %s block", name)
		}
		content := strings.Join(lines[start:i], "\n")
		if inline := line[p+1:]; strings.TrimSpace(inline) != "" {
			content = inline + "\n" + content
		}
		if len(d) >= 10000 {
			return nil, fmt.Errorf("Bruno file exceeds 10000 blocks")
		}
		d = append(d, block{name, content})
	}
	return d, nil
}
func isText(name string) bool {
	return name == "body" || name == "docs" || name == "tests" || name == "example" || strings.HasPrefix(name, "script:") || (strings.HasPrefix(name, "body:") && name != "body:form-urlencoded" && name != "body:multipart-form" && name != "body:file" && name != "body:grpc" && name != "body:ws")
}
func outdent(s string) string {
	lines := strings.Split(strings.TrimLeft(s, "\n"), "\n")
	for i := range lines {
		lines[i] = strings.TrimPrefix(lines[i], "  ")
	}
	return strings.Join(lines, "\n")
}
func (d document) text(name string) string {
	var s string
	for _, b := range d {
		if b.name == name {
			s = b.text
		}
	}
	return outdent(s)
}
func (d document) has(name string) bool {
	for _, b := range d {
		if b.name == name {
			return true
		}
	}
	return false
}
func (d document) pairs(name string) ([]model.KeyValue, error) {
	var result []model.KeyValue
	for _, b := range d {
		if b.name != name {
			continue
		}
		rows, err := parsePairs(b.text)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		result = append(result, rows...)
	}
	return result, nil
}

var annotationLine = regexp.MustCompile(`^@[A-Za-z_][A-Za-z0-9_-]*(?:\(|$)`)

func parsePairs(s string) ([]model.KeyValue, error) {
	lines := strings.Split(s, "\n")
	var rows []model.KeyValue
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// Annotations carry descriptions/types unsupported by Posting. Skip them as
		// metadata, including multiline descriptions; callers issue a diagnostic.
		if annotationLine.MatchString(line) {
			if strings.Count(line, "'''")%2 == 1 {
				for i++; i < len(lines) && !strings.Contains(lines[i], "'''"); i++ {
				}
				if i == len(lines) {
					return nil, fmt.Errorf("unclosed annotation")
				}
			}
			continue
		}
		enabled := true
		if strings.HasPrefix(line, "~") {
			enabled = false
			line = line[1:]
		}
		var key, value string
		if strings.HasPrefix(line, "\"") {
			var b strings.Builder
			j := 1
			for ; j < len(line); j++ {
				if line[j] == '\\' && j+1 < len(line) && line[j+1] == '"' {
					b.WriteByte('"')
					j++
					continue
				}
				if line[j] == '"' {
					break
				}
				b.WriteByte(line[j])
			}
			if j == len(line) {
				return nil, fmt.Errorf("unterminated quoted key")
			}
			key = b.String()
			rest := strings.TrimSpace(line[j+1:])
			if !strings.HasPrefix(rest, ":") {
				return nil, fmt.Errorf("expected colon after key")
			}
			value = strings.TrimSpace(rest[1:])
		} else {
			p := strings.IndexByte(line, ':')
			if p < 0 {
				return nil, fmt.Errorf("expected key: value")
			}
			key = strings.TrimSpace(line[:p])
			value = strings.TrimSpace(line[p+1:])
		}
		if strings.HasPrefix(key, "~") {
			enabled = false
			key = key[1:]
		}
		if key == "" {
			return nil, fmt.Errorf("empty key")
		}
		if strings.HasPrefix(value, "'''") {
			value = value[3:]
			parts := []string{value}
			for !strings.Contains(parts[len(parts)-1], "'''") {
				i++
				if i == len(lines) {
					return nil, fmt.Errorf("unterminated multiline value")
				}
				parts = append(parts, lines[i])
			}
			value = strings.Join(parts, "\n")
			end := strings.Index(value, "'''")
			tail := strings.TrimSpace(value[end+3:])
			if tail != "" && !strings.HasPrefix(tail, "@contentType(") {
				return nil, fmt.Errorf("unexpected multiline suffix")
			}
			parts = strings.Split(value[:end], "\n")
			for j := range parts {
				if len(parts[j]) >= 4 {
					parts[j] = parts[j][4:]
				} else {
					parts[j] = ""
				}
			}
			value = strings.TrimSpace(strings.Join(parts, "\n"))
			if tail != "" {
				value += " " + tail
			}
		} else if value == "[" {
			for i++; i < len(lines) && strings.TrimSpace(lines[i]) != "]"; i++ {
			}
			if i == len(lines) {
				return nil, fmt.Errorf("unterminated list")
			}
			value = ""
		}
		if len(rows) >= 10000 {
			return nil, fmt.Errorf("Bruno block exceeds 10000 rows")
		}
		rows = append(rows, model.KeyValue{Name: key, Value: value, Enabled: enabled})
	}
	return rows, nil
}
func values(rows []model.KeyValue) map[string]string {
	m := map[string]string{}
	for _, r := range rows {
		if r.Enabled {
			m[r.Name] = r.Value
		}
	}
	return m
}
