package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAppliesMigrations(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "fbitcli.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v != schemaVersion {
		t.Fatalf("schema version = %d, want %d", v, schemaVersion)
	}
}
