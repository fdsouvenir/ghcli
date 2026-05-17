// Package paths resolves fbitcli's local state layout.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// Layout is the canonical file layout under a single state directory.
type Layout struct {
	Root        string
	Database    string
	Credentials string
}

// Resolve returns the layout rooted at storeOverride, otherwise at
// $XDG_STATE_HOME/fbitcli or ~/.local/state/fbitcli.
func Resolve(storeOverride string) (Layout, error) {
	root := storeOverride
	if root == "" {
		var err error
		root, err = defaultRoot()
		if err != nil {
			return Layout{}, err
		}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Layout{}, fmt.Errorf("resolve store path: %w", err)
	}
	return Layout{
		Root:        abs,
		Database:    filepath.Join(abs, "fbitcli.db"),
		Credentials: defaultCredentialsPath(),
	}, nil
}

// EnsureDirs creates the store root with private permissions.
func (l Layout) EnsureDirs() error {
	return os.MkdirAll(l.Root, 0o700)
}

func defaultRoot() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "fbitcli"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine home dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "fbitcli"), nil
}

func defaultCredentialsPath() string {
	if p := os.Getenv("FBITCLI_GOOGLE_CREDENTIALS"); p != "" {
		return p
	}
	if p := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); p != "" {
		return p
	}
	if _, err := os.Stat("ghapi-credentials.json"); err == nil {
		return "ghapi-credentials.json"
	}
	return "ghapi-credentials.json"
}
