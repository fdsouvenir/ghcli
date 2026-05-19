package health

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestFileTokenStoreSaveLoadAndDelete(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state", "token.json")
	store := FileTokenStore{Path: path}
	tok := &oauth2.Token{
		AccessToken:  "access-value",
		RefreshToken: "refresh-value",
		Expiry:       time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}

	if err := store.Save(ctx, tok); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("token mode = %o, want 600", got)
	}

	loaded, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AccessToken != tok.AccessToken || loaded.RefreshToken != tok.RefreshToken {
		t.Fatal("loaded token did not match saved token")
	}

	if err := store.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(ctx); err == nil {
		t.Fatal("expected missing token after delete")
	}
}

func TestFileTokenStoreRejectsEmptyToken(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "token.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (FileTokenStore{Path: path}).Load(ctx); err == nil {
		t.Fatal("expected empty token error")
	}
}
