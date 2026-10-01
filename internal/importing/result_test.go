package importing

import (
	"strings"
	"testing"

	"github.com/darrenburns/posting/internal/model"
)

func TestBracedOnlyKeepsWhatSubstituteBracedSends(t *testing.T) {
	values := map[string]string{"id": "WRONG", "ID": "7"}
	for _, raw := range []string{"query($id: ID!)", "a $$ b", "${ID}", "$", "$$$", "x$", "$id$${ID}"} {
		escaped := strings.ReplaceAll(raw, "$", "$$")
		want := model.SubstituteBraced(escaped, model.MapLookup(values))
		if got := model.SubstituteBraced(BracedOnly(escaped), model.MapLookup(values)); got != want {
			t.Errorf("%q: BracedOnly(%q) = %q sends %q, want %q", raw, escaped, BracedOnly(escaped), got, want)
		}
		if want != raw {
			t.Errorf("%q: the escaped text sends %q", raw, want)
		}
	}
	if got := BracedOnly("query($$id: ID!) { a(b: $${X}) }"); got != "query($id: ID!) { a(b: $${X}) }" {
		t.Fatalf("BracedOnly = %q", got)
	}
}
