package curl

import (
	"fmt"
	"strings"

	"github.com/darrenburns/posting/internal/model"
)

func parsePart(s string, literal bool) (model.Part, error) {
	name, value, ok := strings.Cut(s, "=")
	p := model.Part{Type: model.PartText, Name: name, Enabled: true}
	if !ok || name == "" {
		return p, fmt.Errorf("multipart fields need a name=value")
	}
	if literal {
		p.Value = value
		return p, nil
	}
	if strings.HasPrefix(value, "<") || strings.HasPrefix(value, "(") {
		return p, fmt.Errorf("file-content fields and nested multipart aren't supported; use text or @path")
	}
	if strings.HasPrefix(value, "@") {
		p.Type, value = model.PartFile, value[1:]
	}
	content, rest, err := formToken(value)
	if err != nil {
		return p, err
	}
	if p.Type == model.PartFile {
		if content == "" || content == "-" {
			return p, fmt.Errorf("multipart files need a regular file path, not stdin")
		}
		if !strings.HasPrefix(value, `"`) && strings.Contains(content, ",") {
			return p, fmt.Errorf("use separate -F fields for multiple files")
		}
		p.File = content
	} else {
		p.Value = content
	}
	for rest != "" {
		if rest[0] != ';' {
			return p, fmt.Errorf("unsupported multipart suffix %q", rest)
		}
		key, remaining, ok := strings.Cut(rest[1:], "=")
		if !ok {
			return p, fmt.Errorf("invalid multipart attribute %q", rest)
		}
		value, tail, err := formToken(remaining)
		if err != nil {
			return p, err
		}
		switch strings.TrimSpace(key) {
		case "type":
			p.ContentType = value
		case "filename":
			p.Filename = value
		default:
			// MIME parameters after type= belong to the content type.
			if p.ContentType != "" && strings.TrimSpace(key) == "charset" {
				p.ContentType += "; charset=" + value
			} else {
				return p, fmt.Errorf("unsupported multipart attribute %q", key)
			}
		}
		rest = tail
	}
	return p, nil
}

// curl's form grammar has its own quoting inside the shell's quoting.
func formToken(s string) (value, rest string, err error) {
	if !strings.HasPrefix(s, `"`) {
		if i := strings.IndexByte(s, ';'); i >= 0 {
			return strings.TrimSpace(s[:i]), s[i:], nil
		}
		return strings.TrimSpace(s), "", nil
	}
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		if s[i] == '"' {
			return b.String(), s[i+1:], nil
		}
		if s[i] == '\\' && i+1 < len(s) && (s[i+1] == '\\' || s[i+1] == '"') {
			i++
		}
		b.WriteByte(s[i])
	}
	return "", "", fmt.Errorf("unterminated quoted multipart value")
}

func formQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func exportFilePath(root, path string) string {
	if root == "" || len(model.FindVariables(path)) > 0 {
		return path
	}
	return model.UploadPath(root, path)
}
