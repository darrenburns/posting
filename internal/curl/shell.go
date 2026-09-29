package curl

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Split breaks a command line into words the way a POSIX shell would,
// including $'...' strings (which browsers use in "Copy as cURL") and
// backslash-newline line continuations.
func Split(command string) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord := false
	flush := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	s := command
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush()
		case c == '\\':
			if i+1 >= len(s) {
				continue
			}
			next := s[i+1]
			i++
			if next == '\n' {
				continue // Line continuation.
			}
			if next == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				i++
				continue
			}
			word.WriteByte(next)
			inWord = true
		case c == '\'':
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				return nil, fmt.Errorf("unterminated ' quote")
			}
			word.WriteString(s[i+1 : i+1+end])
			inWord = true
			i += end + 1
		case c == '$' && i+1 < len(s) && s[i+1] == '\'':
			consumed, text, err := ansiC(s[i+2:])
			if err != nil {
				return nil, err
			}
			word.WriteString(text)
			inWord = true
			i += 1 + consumed
		case c == '"':
			j := i + 1
			for ; j < len(s) && s[j] != '"'; j++ {
				if s[j] == '\\' && j+1 < len(s) {
					switch s[j+1] {
					case '"', '\\', '$', '`':
						word.WriteByte(s[j+1])
						j++
						continue
					case '\n':
						j++
						continue
					}
				}
				word.WriteByte(s[j])
			}
			if j >= len(s) {
				return nil, fmt.Errorf(`unterminated " quote`)
			}
			inWord = true
			i = j
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	flush()
	return words, nil
}

// ansiC decodes the body of a $'...' string. It returns how many bytes it
// consumed, including the closing quote.
func ansiC(s string) (int, string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' {
			return i + 1, b.String(), nil
		}
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch e := s[i]; e {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'e', 'E':
			b.WriteByte(0x1b)
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '\\', '\'', '"', '?':
			b.WriteByte(e)
		case 'x', 'u', 'U':
			digits := map[byte]int{'x': 2, 'u': 4, 'U': 8}[e]
			j := i + 1
			for j < len(s) && j < i+1+digits && isHex(s[j]) {
				j++
			}
			if j == i+1 {
				b.WriteByte('\\')
				b.WriteByte(e)
				continue
			}
			n, _ := strconv.ParseUint(s[i+1:j], 16, 32)
			if e == 'x' {
				b.WriteByte(byte(n))
			} else {
				b.WriteRune(rune(n))
			}
			i = j - 1
		default:
			if e >= '0' && e <= '7' {
				j := i
				for j < len(s) && j < i+3 && s[j] >= '0' && s[j] <= '7' {
					j++
				}
				n, _ := strconv.ParseUint(s[i:j], 8, 8)
				b.WriteByte(byte(n))
				i = j - 1
				continue
			}
			b.WriteByte('\\')
			b.WriteByte(e)
		}
	}
	return 0, "", fmt.Errorf("unterminated $' quote")
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// Quote makes s a single shell word, quoting only when needed.
func Quote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n\r'\"\\$`!*?&;|<>(){}[]#~=%^") && utf8.ValidString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
