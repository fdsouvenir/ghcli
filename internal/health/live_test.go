package health

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveGetIdentity(t *testing.T) {
	if os.Getenv("FBITCLI_LIVE_TESTS") != "1" {
		t.Skip("set FBITCLI_LIVE_TESTS=1 after OAuth login to run live Google Health API tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	creds, err := LoadCredentials("../../ghapi-credentials.json")
	if err != nil {
		t.Fatal(err)
	}
	src, err := TokenSource(ctx, creds.Config(""), DefaultVaultTokenStore())
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
