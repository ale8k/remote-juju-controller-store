package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current session identity",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := requireSession()
		if err != nil {
			return err
		}
		tok, _, err := new(jwt.Parser).ParseUnverified(s.Token, jwt.MapClaims{})
		if err != nil {
			return fmt.Errorf("parse session token: %w", err)
		}
		claims, _ := tok.Claims.(jwt.MapClaims)
		sub, _ := claims["sub"].(string)
		email, _ := claims["email"].(string)
		expStr := ""
		if exp, ok := claims["exp"].(float64); ok {
			expStr = time.Unix(int64(exp), 0).Format(time.RFC3339)
		}
		out := map[string]string{
			"addr":       s.Addr,
			"sub":        sub,
			"email":      email,
			"expires_at": expStr,
			"namespace":  strings.TrimSpace(s.Namespace),
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	},
}
