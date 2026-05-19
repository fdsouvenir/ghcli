package health

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCredentialsJSON = `{
  "installed": {
    "client_id": "client-id.example",
    "client_secret": "client-secret-value",
    "redirect_uris": ["http://localhost:8080"]
  }
}`

func TestLoadCredentialSourcePrecedence(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "client.json")
	if err := os.WriteFile(filePath, []byte(testCredentialsJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GHCLI_GOOGLE_CREDENTIALS", filePath)
	t.Setenv("GHCLI_GOOGLE_CREDENTIALS_JSON", `{"installed":{"client_id":"inline","client_secret":"inline-secret"}}`)

	res, err := LoadCredentialSource("")
	if err != nil {
		t.Fatal(err)
	}
	if res.Source.Type != "file" {
		t.Fatalf("source type = %q, want file", res.Source.Type)
	}
	if got := res.Credentials.Installed.ClientID; got != "client-id.example" {
		t.Fatalf("client id = %q, want file credential", got)
	}
}

func TestLoadCredentialSourceInlineJSON(t *testing.T) {
	t.Setenv("GHCLI_GOOGLE_CREDENTIALS", "")
	t.Setenv("GHCLI_GOOGLE_CREDENTIALS_JSON", testCredentialsJSON)

	res, err := LoadCredentialSource("")
	if err != nil {
		t.Fatal(err)
	}
	if res.Source.Type != "inline_json" {
		t.Fatalf("source type = %q, want inline_json", res.Source.Type)
	}
	if res.Credentials.Installed.TokenURI == "" || res.Credentials.Installed.AuthURI == "" {
		t.Fatal("default OAuth endpoint URI not populated")
	}
}

func TestLoadCredentialSourceMissing(t *testing.T) {
	t.Setenv("GHCLI_GOOGLE_CREDENTIALS", "")
	t.Setenv("GHCLI_GOOGLE_CREDENTIALS_JSON", "")

	_, err := LoadCredentialSource("")
	if err == nil {
		t.Fatal("expected missing credentials error")
	}
	if !strings.Contains(err.Error(), "GHCLI_GOOGLE_CREDENTIALS_JSON") {
		t.Fatalf("error %q does not include setup guidance", err)
	}
}

func TestLoadCredentialSourceDoesNotUseImplicitRepoFile(t *testing.T) {
	t.Setenv("GHCLI_GOOGLE_CREDENTIALS", "")
	t.Setenv("GHCLI_GOOGLE_CREDENTIALS_JSON", "")
	t.Chdir(t.TempDir())
	if err := os.WriteFile("ghapi-credentials.json", []byte(testCredentialsJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := LoadCredentialSource("")
	if err == nil {
		t.Fatal("expected missing credentials error")
	}
	if res.Source.Type != "missing" {
		t.Fatalf("source type = %q, want missing", res.Source.Type)
	}
}

func TestParseCredentialsRedactsSecretOnInvalidShape(t *testing.T) {
	_, err := ParseCredentials([]byte(`{"installed":{"client_secret":"do-not-leak"}}`), "test-json")
	if err == nil {
		t.Fatal("expected invalid credential shape error")
	}
	if strings.Contains(err.Error(), "do-not-leak") {
		t.Fatalf("error leaked secret: %q", err)
	}
}
