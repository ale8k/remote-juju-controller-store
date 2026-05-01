package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

var nsCmd = &cobra.Command{Use: "ns", Short: "Namespace operations"}

var nsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create namespace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := requireSession()
		if err != nil {
			return err
		}
		resp, err := authedRequest(s, http.MethodPost, "/namespaces", map[string]string{"name": args[0]})
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if err := mustStatus(resp, http.StatusCreated); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "namespace %s created\n", args[0])
		return nil
	},
}

var nsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List accessible namespaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := requireSession()
		if err != nil {
			return err
		}
		resp, err := authedRequest(s, http.MethodGet, "/namespaces", nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if err := mustStatus(resp, http.StatusOK); err != nil {
			return err
		}
		b, _ := io.ReadAll(resp.Body)
		var arr []map[string]any
		if err := json.Unmarshal(b, &arr); err != nil {
			return err
		}
		for _, n := range arr {
			name, _ := n["name"].(string)
			fmt.Fprintln(cmd.OutOrStdout(), name)
		}
		return nil
	},
}

var nsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete namespace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := requireSession()
		if err != nil {
			return err
		}
		resp, err := authedRequest(s, http.MethodDelete, "/namespaces/"+args[0], nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if err := mustStatus(resp, http.StatusNoContent); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "namespace %s deleted\n", args[0])
		if s.Namespace == args[0] {
			s.Namespace = ""
			_ = saveSession(*s)
		}
		return nil
	},
}

var useCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set active namespace in session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := requireSession()
		if err != nil {
			return err
		}
		name := strings.TrimSpace(args[0])
		s.Namespace = name
		if err := saveSession(*s); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "active namespace: %s\n", name)
		return nil
	},
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Show current server and namespace",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := requireSession()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "server: %s\n", s.Addr)
		if s.Namespace == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "namespace: <unset>")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "namespace: %s\n", s.Namespace)
		}
		return nil
	},
}

func init() {
	nsCmd.AddCommand(nsCreateCmd)
	nsCmd.AddCommand(nsListCmd)
	nsCmd.AddCommand(nsDeleteCmd)
}
