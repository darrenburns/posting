package skills

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSkillsFollowTheAgentSkillsSpec checks what agents need to load a
// skill: frontmatter with a name matching its directory and a description
// within the limit.
func TestSkillsFollowTheAgentSkillsSpec(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		found++
		data, err := os.ReadFile(e.Name() + "/SKILL.md")
		if err != nil {
			t.Errorf("%s: %v", e.Name(), err)
			continue
		}
		parts := strings.SplitN(string(data), "---\n", 3)
		if len(parts) != 3 || parts[0] != "" {
			t.Errorf("%s: SKILL.md doesn't start with frontmatter", e.Name())
			continue
		}
		var meta struct{ Name, Description string }
		if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
			t.Errorf("%s: frontmatter: %v", e.Name(), err)
			continue
		}
		if meta.Name != e.Name() || !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(meta.Name) || len(meta.Name) > 64 {
			t.Errorf("%s: name %q must match the directory, in lowercase words joined by hyphens", e.Name(), meta.Name)
		}
		if meta.Description == "" || len(meta.Description) > 1024 {
			t.Errorf("%s: description is %d characters, want 1 to 1024", e.Name(), len(meta.Description))
		}
	}
	if found == 0 {
		t.Error("no skills found")
	}
}

func TestUsingPostingIsEmbedded(t *testing.T) {
	data, err := os.ReadFile("using-posting/SKILL.md")
	if err != nil || string(data) != UsingPosting {
		t.Errorf("the embedded skill differs from using-posting/SKILL.md (%v)", err)
	}
}
