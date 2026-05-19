package health

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// OAuthCredentials is the installed-app Google OAuth client JSON shape.
type OAuthCredentials struct {
	Installed struct {
		ClientID                string   `json:"client_id"`
		ClientSecret            string   `json:"client_secret"`
		AuthURI                 string   `json:"auth_uri"`
		TokenURI                string   `json:"token_uri"`
		AuthProviderX509CertURL string   `json:"auth_provider_x509_cert_url"`
		ProjectID               string   `json:"project_id"`
		RedirectURIs            []string `json:"redirect_uris"`
	} `json:"installed"`
}

// CredentialSource describes where credentials were loaded from without
// including secret material.
type CredentialSource struct {
	Type  string `json:"type"`
	Label string `json:"label"`
}

// CredentialLoadResult includes parsed credentials and a non-secret source.
type CredentialLoadResult struct {
	Credentials OAuthCredentials
	Source      CredentialSource
}

// LoadCredentials reads Google OAuth installed-app credentials from a file.
func LoadCredentials(path string) (OAuthCredentials, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return OAuthCredentials{}, fmt.Errorf("read credentials %s: %w", path, err)
	}
	return ParseCredentials(b, path)
}

// LoadCredentialSource resolves credentials from an explicit file path, the
// GHCLI_GOOGLE_CREDENTIALS file path env var, or inline
// GHCLI_GOOGLE_CREDENTIALS_JSON.
func LoadCredentialSource(pathOverride string) (CredentialLoadResult, error) {
	if pathOverride != "" {
		creds, err := LoadCredentials(pathOverride)
		return CredentialLoadResult{
			Credentials: creds,
			Source:      CredentialSource{Type: "file", Label: pathOverride},
		}, err
	}
	if path := os.Getenv("GHCLI_GOOGLE_CREDENTIALS"); path != "" {
		creds, err := LoadCredentials(path)
		return CredentialLoadResult{
			Credentials: creds,
			Source:      CredentialSource{Type: "file", Label: "GHCLI_GOOGLE_CREDENTIALS"},
		}, err
	}
	if raw := os.Getenv("GHCLI_GOOGLE_CREDENTIALS_JSON"); raw != "" {
		creds, err := ParseCredentials([]byte(raw), "GHCLI_GOOGLE_CREDENTIALS_JSON")
		return CredentialLoadResult{
			Credentials: creds,
			Source:      CredentialSource{Type: "inline_json", Label: "GHCLI_GOOGLE_CREDENTIALS_JSON"},
		}, err
	}
	return CredentialLoadResult{
		Source: CredentialSource{Type: "missing", Label: "unset"},
	}, errors.New("missing Google OAuth credentials; set --credentials, GHCLI_GOOGLE_CREDENTIALS, or GHCLI_GOOGLE_CREDENTIALS_JSON")
}

// ParseCredentials parses Google OAuth installed-app credentials.
func ParseCredentials(b []byte, label string) (OAuthCredentials, error) {
	var creds OAuthCredentials
	if err := json.Unmarshal(b, &creds); err != nil {
		return creds, fmt.Errorf("parse credentials from %s: %w", label, err)
	}
	if creds.Installed.ClientID == "" || creds.Installed.ClientSecret == "" {
		return creds, fmt.Errorf("credentials from %s missing installed.client_id/client_secret", label)
	}
	if creds.Installed.AuthURI == "" {
		creds.Installed.AuthURI = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	if creds.Installed.TokenURI == "" {
		creds.Installed.TokenURI = "https://oauth2.googleapis.com/token"
	}
	return creds, nil
}

// Config returns an OAuth2 config for Google Health read-only access.
func (c OAuthCredentials) Config(redirectURI string) *oauth2.Config {
	if redirectURI == "" {
		redirectURI = c.DefaultRedirectURI()
	}
	return &oauth2.Config{
		ClientID:     c.Installed.ClientID,
		ClientSecret: c.Installed.ClientSecret,
		RedirectURL:  redirectURI,
		Scopes:       ReadOnlyScopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  c.Installed.AuthURI,
			TokenURL: c.Installed.TokenURI,
		},
	}
}

// DefaultRedirectURI picks the first loopback redirect URI when available.
func (c OAuthCredentials) DefaultRedirectURI() string {
	for _, u := range c.Installed.RedirectURIs {
		parsed, err := url.Parse(u)
		if err == nil && parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1") {
			return u
		}
	}
	if len(c.Installed.RedirectURIs) > 0 {
		return c.Installed.RedirectURIs[0]
	}
	return "http://localhost:8080"
}

// TokenStore persists OAuth tokens.
type TokenStore interface {
	Load(context.Context) (*oauth2.Token, error)
	Save(context.Context, *oauth2.Token) error
	Describe() string
}

// FileTokenStore stores OAuth tokens in a private JSON file.
type FileTokenStore struct {
	Path string
}

// Describe returns a non-secret token location.
func (f FileTokenStore) Describe() string {
	return "file:" + f.Path
}

// EnsureReady creates the token directory with private permissions.
func (f FileTokenStore) EnsureReady(context.Context) error {
	dir := filepath.Dir(f.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".token-check-*.tmp")
	if err != nil {
		return fmt.Errorf("check token store %s: %w", f.Describe(), err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Remove(tmpPath)
}

// Load reads and parses a token file.
func (f FileTokenStore) Load(context.Context) (*oauth2.Token, error) {
	b, err := os.ReadFile(f.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("token not found at %s", f.Describe())
		}
		return nil, fmt.Errorf("read token from %s: %w", f.Describe(), err)
	}
	var tok oauth2.Token
	if err := json.Unmarshal(b, &tok); err != nil {
		return nil, fmt.Errorf("parse token from %s: %w", f.Describe(), err)
	}
	if tok.AccessToken == "" && tok.RefreshToken == "" {
		return nil, fmt.Errorf("token in %s has no access_token or refresh_token", f.Describe())
	}
	return &tok, nil
}

// Save writes a token file atomically with private permissions.
func (f FileTokenStore) Save(ctx context.Context, tok *oauth2.Token) error {
	if err := f.EnsureReady(ctx); err != nil {
		return err
	}
	b, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(f.Path), ".token-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, f.Path); err != nil {
		return fmt.Errorf("save token to %s: %w", f.Describe(), err)
	}
	return os.Chmod(f.Path, 0o600)
}

// Delete removes the local token file.
func (f FileTokenStore) Delete(context.Context) error {
	if err := os.Remove(f.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove token at %s: %w", f.Describe(), err)
	}
	return nil
}

// TokenSource returns an oauth2 token source that persists refreshed tokens.
func TokenSource(ctx context.Context, conf *oauth2.Config, store TokenStore) (oauth2.TokenSource, error) {
	tok, err := store.Load(ctx)
	if err != nil {
		return nil, err
	}
	return &savingTokenSource{
		base:  conf.TokenSource(ctx, tok),
		store: store,
		last:  tok,
		ctx:   ctx,
	}, nil
}

type savingTokenSource struct {
	base  oauth2.TokenSource
	store TokenStore
	last  *oauth2.Token
	ctx   context.Context
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.base.Token()
	if err != nil {
		return nil, err
	}
	if s.last == nil || tok.AccessToken != s.last.AccessToken || tok.RefreshToken != s.last.RefreshToken || !tok.Expiry.Equal(s.last.Expiry) {
		if err := s.store.Save(s.ctx, tok); err != nil {
			return nil, err
		}
		s.last = tok
	}
	return tok, nil
}

// AuthCodeFlow prints an authorization URL and exchanges the resulting code.
func AuthCodeFlow(ctx context.Context, conf *oauth2.Config, store TokenStore, loopback bool) error {
	state, err := randomState()
	if err != nil {
		return err
	}

	var code string
	if loopback {
		code, err = loopbackCode(ctx, conf, state)
		if err != nil {
			return err
		}
	} else {
		authURL := conf.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
		fmt.Println("Open this URL in a browser and approve Google Health access:")
		fmt.Println(authURL)
		fmt.Println()
		fmt.Println("Paste the full redirect URL or just the code:")
		var input string
		if _, err := fmt.Scanln(&input); err != nil {
			return err
		}
		code, err = parseCode(input)
		if err != nil {
			return err
		}
	}

	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("exchange auth code: %w", err)
	}
	if err := store.Save(ctx, tok); err != nil {
		return err
	}
	fmt.Printf("Token saved to %s\n", store.Describe())
	return nil
}

func loopbackCode(ctx context.Context, conf *oauth2.Config, state string) (string, error) {
	u, err := url.Parse(conf.RedirectURL)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1") {
		return "", fmt.Errorf("loopback requires http://localhost or http://127.0.0.1 redirect URI")
	}
	if u.Port() == "" {
		return "", fmt.Errorf("loopback requires an explicit port in the redirect URI; use copy/paste mode for %s", conf.RedirectURL)
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	ln, err := net.Listen("tcp", u.Host)
	if err != nil {
		return "", err
	}
	defer ln.Close()

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second}
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("state"); got != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- errors.New("OAuth state mismatch")
			return
		}
		c := r.URL.Query().Get("code")
		if c == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- errors.New("OAuth redirect missing code")
			return
		}
		fmt.Fprintln(w, "OK. You can close this tab and return to ghcli.")
		codeCh <- c
	})
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	defer srv.Shutdown(context.Background())

	fmt.Println("Open this URL in a browser and approve Google Health access:")
	fmt.Println(conf.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce))
	fmt.Println()
	fmt.Println("Waiting for OAuth redirect on " + conf.RedirectURL)

	select {
	case c := <-codeCh:
		return c, nil
	case err := <-errCh:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(3 * time.Minute):
		return "", errors.New("timed out waiting for OAuth redirect")
	}
}

func parseCode(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("empty code")
	}
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		u, err := url.Parse(input)
		if err != nil {
			return "", err
		}
		code := u.Query().Get("code")
		if code == "" {
			return "", errors.New("redirect URL missing code parameter")
		}
		return code, nil
	}
	return input, nil
}

func randomState() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
