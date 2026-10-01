package bruno

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/env"
	"github.com/darrenburns/posting/internal/importing"
	"github.com/darrenburns/posting/internal/model"
)

func TestImportedVariableScopesRoundTrip(t *testing.T) {
	cases := []struct{ name, coll, folder, environ, local, want string }{
		{"EnvironmentOverridesCollectionAlias", "vars:pre-request {\n host: https://collection.example\n baseUrl: {{host}}/api\n}\n", "vars:pre-request {\n host: https://folder.example\n}\n", "vars {\n baseUrl: https://selected.example/v2\n}\n", "", "https://selected.example/v2/ping"},
		{"EnvironmentAliasUsesFolderDependency", "", "vars:pre-request {\n host: https://folder.example\n}\n", "vars {\n host: https://environment.example\n baseUrl: {{host}}/api\n}\n", "", "https://folder.example/api/ping"},
		{"SecretInLocalFileResolvesAlias", "", "", "vars {\n baseUrl: https://example.test/{{token}}\n}\nvars:secret [\n token\n]\n", "token=correct-secret\n", "https://example.test/correct-secret/ping"},
		{"CollectionSecretAliasResolves", "vars:pre-request {\n baseUrl: https://example.test/{{token}}\n}\n", "", "vars:secret [\n token\n]\n", "token=correct-secret\n", "https://example.test/correct-secret/ping"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{"bruno.json": `{"name":"Review","type":"collection"}`, "A/ping.bru": "get {\n url: {{baseUrl}}/ping\n}\n", "environments/staging.bru": tc.environ}
			if tc.coll != "" {
				files["collection.bru"] = tc.coll
			}
			if tc.folder != "" {
				files["A/folder.bru"] = tc.folder
			}
			result, err := Load(writeCollection(t, files))
			if err != nil {
				t.Fatal(err)
			}
			dest := t.TempDir()
			written, err := importing.Write(result, dest)
			if err != nil {
				t.Fatal(err)
			}
			if tc.local != "" {
				if err := os.WriteFile(filepath.Join(dest, "staging.local.env"), []byte(tc.local), 0600); err != nil {
					t.Fatal(err)
				}
			}
			loaded, err := env.Load(env.Stack(dest, "staging"))
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dest, written.Files[0]))
			if err != nil {
				t.Fatal(err)
			}
			req, err := collection.ParseRequest(data, written.Files[0])
			if err != nil {
				t.Fatal(err)
			}
			resolved, err := model.Resolve(req, model.MapLookup(model.Values(loaded.Variables)))
			if err != nil {
				t.Fatal(err)
			}

			if resolved.URL != tc.want {
				t.Errorf("expected %q; actual %q", tc.want, resolved.URL)
			}
		})
	}
}

func TestEnvironmentMetadataAndSecretListSyntax(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"EnvironmentColor", "color: cyan\nvars {\n host: https://example.test\n}\n"},
		{"InlineSecretList", "vars:secret [token]\n"},
		{"IndentedSecretListClose", "vars:secret [\n token\n ]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(writeCollection(t, map[string]string{"bruno.json": `{"name":"Review","type":"collection"}`, "ping.bru": "get {\n url: https://example.test\n}\n", "environments/staging.bru": tc.content}))
			if err != nil {
				t.Errorf("valid Bruno environment rejects entire collection: %v", err)
			}
		})
	}
}
