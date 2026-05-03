package cmd

import (
	"fmt"
	"strings"

	rcsclient "github.com/ale8k/remote-juju-controller-store/pkg/client"
	"github.com/spf13/cobra"
)

const (
	ansiGreen = "\033[32m"
	ansiReset = "\033[0m"
)

func fetchAccessibleNamespaces(cmd *cobra.Command, s *Session) ([]rcsclient.NamespaceResponse, error) {
	api, err := authedAPIClient(s)
	if err != nil {
		return nil, err
	}
	resp, err := api.ListNamespacesWithResponse(cmd.Context())
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 || resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected status %s: %s", resp.Status(), strings.TrimSpace(string(resp.Body)))
	}
	return *resp.JSON200, nil
}

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
		api, err := authedAPIClient(s)
		if err != nil {
			return err
		}
		resp, err := api.CreateNamespaceWithResponse(cmd.Context(), rcsclient.CreateNamespaceRequest{Name: args[0]})
		if err != nil {
			return err
		}
		if resp.StatusCode() != 201 {
			return fmt.Errorf("unexpected status %s: %s", resp.Status(), strings.TrimSpace(string(resp.Body)))
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
		arr, err := fetchAccessibleNamespaces(cmd, s)
		if err != nil {
			return err
		}
		for _, n := range arr {
			line := n.Name
			if n.Name == s.Namespace {
				line = line + " *"
				line = ansiGreen + line + ansiReset
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
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
		api, err := authedAPIClient(s)
		if err != nil {
			return err
		}
		resp, err := api.DeleteNamespaceWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if resp.StatusCode() != 204 {
			return fmt.Errorf("unexpected status %s: %s", resp.Status(), strings.TrimSpace(string(resp.Body)))
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
		if name == "" {
			return fmt.Errorf("namespace name is required")
		}

		namespaces, err := fetchAccessibleNamespaces(cmd, s)
		if err != nil {
			return err
		}
		allowed := false
		for _, ns := range namespaces {
			if ns.Name == name {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("namespace %q does not exist or you are not a member", name)
		}

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
