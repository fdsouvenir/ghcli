package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
)

func TestOpenAppliesMigrations(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "ghcli.db"))
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

func TestOpenSerializesConcurrentMigrations(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "ghcli.db")
	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st, err := Open(ctx, dbPath)
			if err != nil {
				errs <- err
				return
			}
			defer st.Close()
			if v, err := st.SchemaVersion(ctx); err != nil {
				errs <- err
			} else if v != schemaVersion {
				errs <- &versionError{got: v, want: schemaVersion}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

type versionError struct {
	got  int
	want int
}

func (e *versionError) Error() string {
	return "schema version mismatch"
}
