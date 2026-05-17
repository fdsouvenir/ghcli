package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/fdsouvenir/fbitcli/internal/health"
	"github.com/fdsouvenir/fbitcli/internal/output"
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
			creds, err := health.LoadCredentials(layout.Credentials)
			if err != nil {
				return err
			}
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			if err := health.DefaultVaultTokenStore().EnsureEntry(ctx); err != nil {
				return err
			}
			report := map[string]any{
				"credentials_path": layout.Credentials,
				"project_id":       creds.Installed.ProjectID,
				"redirect_uri":     creds.DefaultRedirectURI(),
				"scopes":           health.ReadOnlyScopes,
				"token_store":      health.DefaultVaultTokenStore().Describe(),
			}
			if flags.jsonOut {
				return output.JSON(cmd.OutOrStdout(), report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "credentials: %s\n", layout.Credentials)
			fmt.Fprintf(cmd.OutOrStdout(), "project:     %s\n", creds.Installed.ProjectID)
			fmt.Fprintf(cmd.OutOrStdout(), "redirect:    %s\n", creds.DefaultRedirectURI())
			fmt.Fprintf(cmd.OutOrStdout(), "token store: %s\n", health.DefaultVaultTokenStore().Describe())
			return nil
		},
	}
}

func authLoginCmd() *cobra.Command {
	var loopback bool
	c := &cobra.Command{
		Use:   "login",
		Short: "Run Google OAuth login and save token to KeePassXC",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalContext(context.Background())
			defer cancel()
			layout, err := resolveLayout()
			if err != nil {
				return err
			}
			creds, err := health.LoadCredentials(layout.Credentials)
			if err != nil {
				return err
			}
			return health.AuthCodeFlow(ctx, creds.Config(""), health.DefaultVaultTokenStore(), loopback)
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
			store := health.DefaultVaultTokenStore()
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
		Short: "Explain how to remove local token state",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(os.Stderr, "Remove the %s attachment from KeePassXC to clear local token state.\n", health.DefaultVaultTokenStore().Describe())
			return nil
		},
	}
}
