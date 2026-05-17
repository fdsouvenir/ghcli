package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoLegacyFitbitWebAPIHosts(t *testing.T) {
	forbidden := []string{
		"api." + "fitbit.com",
		"www." + "fitbit.com/" + "oauth2",
		"fitbit.com/" + "oauth2",
	}
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".cache":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) == "no_legacy_fitbit_test.go" {
			return nil
		}
		if strings.HasSuffix(path, ".sum") || strings.HasSuffix(path, "ghapi-credentials.json") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(b)
		for _, bad := range forbidden {
			if strings.Contains(text, bad) {
				t.Fatalf("%s contains forbidden legacy Fitbit Web API reference %q", path, bad)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
