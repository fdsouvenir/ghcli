package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/fdsouvenir/ghcli/internal/health"
	"github.com/fdsouvenir/ghcli/internal/output"
)

func authCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "auth",
		Short: "Authorize Google Health API access",
	}
	c.AddCommand(authSetupCmd())
	c.AddCommand(authLoginCmd())
	c.AddCommand(authStatusCmd())
	c.AddCommand(authRevokeLocalCmd())
	return c
}

func authSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Validate local Google OAuth credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			layout, err := resolveLayout()
			if err != nil {
				return err
			}
			res, err := health.LoadCredentialSource(flags.credentials)
			if err != nil {
				return err
			}
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			store := tokenStore(layout)
			if err := store.EnsureReady(ctx); err != nil {
				return err
			}
			report := map[string]any{
				"credentials_source_type": res.Source.Type,
				"credentials_source":      res.Source.Label,
				"project_id":              res.Credentials.Installed.ProjectID,
				"redirect_uri":            res.Credentials.DefaultRedirectURI(),
				"scopes":                  health.ReadOnlyScopes,
				"token_store":             store.Describe(),
			}
			if flags.jsonOut {
				return output.JSON(cmd.OutOrStdout(), report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "credentials: %s %s\n", res.Source.Type, res.Source.Label)
			fmt.Fprintf(cmd.OutOrStdout(), "project:     %s\n", res.Credentials.Installed.ProjectID)
			fmt.Fprintf(cmd.OutOrStdout(), "redirect:    %s\n", res.Credentials.DefaultRedirectURI())
			fmt.Fprintf(cmd.OutOrStdout(), "token store: %s\n", store.Describe())
			return nil
		},
	}
}

func authLoginCmd() *cobra.Command {
	var loopback bool
	c := &cobra.Command{
		Use:   "login",
		Short: "Run Google OAuth login and save token locally",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			layout, err := resolveLayout()
			if err != nil {
				return err
			}
			res, err := health.LoadCredentialSource(flags.credentials)
			if err != nil {
				return err
			}
			return health.AuthCodeFlow(ctx, res.Credentials.Config(""), tokenStore(layout), loopback)
		},
	}
	c.Flags().BoolVar(&loopback, "loopback", false, "listen on the configured loopback redirect URI")
	return c
}

func authStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show local token status without printing token values",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			layout, err := resolveLayout()
			if err != nil {
				return err
			}
			store := tokenStore(layout)
			tok, err := store.Load(ctx)
			report := map[string]any{
				"token_store": store.Describe(),
				"present":     err == nil,
			}
			if err != nil {
				report["error"] = err.Error()
			} else {
				report["has_refresh_token"] = tok.RefreshToken != ""
				report["expiry"] = tok.Expiry
				report["expired"] = !tok.Expiry.IsZero() && time.Now().After(tok.Expiry)
			}
			if flags.jsonOut {
				return output.JSON(cmd.OutOrStdout(), report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "token store:       %s\n", store.Describe())
			fmt.Fprintf(cmd.OutOrStdout(), "token present:     %v\n", err == nil)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "issue:             %v\n", err)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "has refresh token: %v\n", tok.RefreshToken != "")
			if !tok.Expiry.IsZero() {
				fmt.Fprintf(cmd.OutOrStdout(), "expires:           %s\n", tok.Expiry.Format(time.RFC3339))
			}
			return nil
		},
	}
}

func authRevokeLocalCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke-local",
		Short: "Remove local token state",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			layout, err := resolveLayout()
			if err != nil {
				return err
			}
			store := tokenStore(layout)
			if err := store.Delete(ctx); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Removed local token state from %s.\n", store.Describe())
			return nil
		},
	}
}
