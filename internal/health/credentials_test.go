package health

import (
	"os"
	"testing"
)

func TestLoadLocalCredentialsShape(t *testing.T) {
	path := "../../ghapi-credentials.json"
	if _, err := os.Stat(path); err != nil {
		t.Skip("local Google Health credentials file not present")
	}
	creds, err := LoadCredentials(path)
	if err != nil {
		t.Fatal(err)
	}
	if creds.Installed.ClientID == "" {
		t.Fatal("missing installed.client_id")
	}
	if creds.Installed.ClientSecret == "" {
		t.Fatal("missing installed.client_secret")
	}
	if creds.Installed.TokenURI == "" || creds.Installed.AuthURI == "" {
		t.Fatal("missing OAuth endpoint URI")
	}
}
