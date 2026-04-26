package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the currently logged-in user",
	Long:  `Displays the subject and server address from the active RCS session.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := loadSession()
		if err != nil {
			return fmt.Errorf("load session: %w", err)
		}
		if session == nil {
			fmt.Fprintln(cmd.OutOrStdout(), "Not logged in. Run: rcs login <addr>")
			return nil
		}

		// Parse the RCS JWT without verifying — we just want to read the claims for display.
		parser := jwt.NewParser(jwt.WithoutClaimsValidation())
		token, _, err := parser.ParseUnverified(session.Token, jwt.MapClaims{})
		if err != nil || !strings.HasPrefix(token.Method.Alg(), "RS") {
			return fmt.Errorf("invalid session token")
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return fmt.Errorf("invalid session token claims")
		}

		// Prefer email over the raw OIDC UUID sub for display.
		user, _ := claims["email"].(string)
		if user == "" {
			user, _ = claims["sub"].(string)
		}

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Server: %s\n", session.Addr)
		fmt.Fprintf(out, "User:   %s\n", user)
		if exp, ok := claims["exp"].(float64); ok {
			expiry := time.Unix(int64(exp), 0)
			now := time.Now()
			fmt.Fprintf(out, "Expiry: %s\n", expiry.Format("2006-01-02 15:04:05 MST"))
			if expiry.After(now) {
				fmt.Fprintf(out, "In:     %s\n", formatRelativeDuration(expiry.Sub(now)))
			} else {
				fmt.Fprintf(out, "Ago:    %s\n", formatRelativeDuration(now.Sub(expiry)))
			}
		}
		return nil
	},
}

func formatRelativeDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}

	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	mins := d / time.Minute

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
