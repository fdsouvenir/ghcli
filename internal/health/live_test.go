package health

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLiveGetIdentity(t *testing.T) {
	if os.Getenv("GHCLI_LIVE_TESTS") != "1" {
		t.Skip("set GHCLI_LIVE_TESTS=1 after OAuth login to run live Google Health API tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := LoadCredentialSource("")
	if err != nil {
		t.Fatal(err)
	}
	src, err := TokenSource(ctx, res.Credentials.Config(""), FileTokenStore{Path: liveTokenPath(t)})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := NewClient(ctx, src).GetIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func liveTokenPath(t *testing.T) string {
	if p := os.Getenv("GHCLI_LIVE_TOKEN"); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "ghcli", "token.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(home, ".local", "state", "ghcli", "token.json")
}
