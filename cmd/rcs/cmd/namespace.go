package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

// nsCmd is the parent for all namespace sub-commands.
var nsCmd = &cobra.Command{
	Use:   "ns",
	Short: "Manage namespaces",
}

var nsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new namespace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		sess, err := requireSession()
		if err != nil {
			return err
		}

		body, _ := json.Marshal(map[string]string{"name": name})
		resp, err := authedRequest(cmd.Context(), sess, http.MethodPost, "/namespaces", body)
		if err != nil {
			return fmt.Errorf("create namespace: %w", err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusCreated:
			var result struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Namespace %q created (id: %s)\n", result.Name, result.ID)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Namespace %q created.\n", name)
			}
		case http.StatusConflict:
			return fmt.Errorf("namespace %q already exists", name)
		default:
			return fmt.Errorf("create namespace: server returned %d", resp.StatusCode)
		}
		return nil
	},
}

var nsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List namespaces you are a member of",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		sess, err := requireSession()
		if err != nil {
			return err
		}

		resp, err := authedRequest(cmd.Context(), sess, http.MethodGet, "/namespaces", nil)
		if err != nil {
			return fmt.Errorf("list namespaces: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("list namespaces: server returned %d", resp.StatusCode)
		}

		var nsList []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			OwnerID string `json:"owner_id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&nsList); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		if len(nsList) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No namespaces found.")
			return nil
		}

		for _, ns := range nsList {
			marker := ""
			if ns.Name == sess.Namespace {
				marker = " *"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s%s\n", ns.Name, marker)
		}
		return nil
	},
}

var nsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a namespace (owner only, destructive)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		sess, err := requireSession()
		if err != nil {
			return err
		}

		resp, err := authedRequest(cmd.Context(), sess, http.MethodDelete, "/namespaces/"+name, nil)
		if err != nil {
			return fmt.Errorf("delete namespace: %w", err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusNoContent:
			// success
		case http.StatusNotFound:
			return fmt.Errorf("namespace %q not found", name)
		case http.StatusForbidden:
			return fmt.Errorf("only the namespace owner can delete it")
		default:
			return fmt.Errorf("delete namespace: server returned %d", resp.StatusCode)
		}

		// If the deleted namespace was the active one, clear it from session.
		if sess.Namespace == name {
			sess.Namespace = ""
			if err := saveSession(sess); err != nil {
				return fmt.Errorf("clear namespace from session: %w", err)
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Namespace %q deleted.\n", name)
		return nil
	},
}

// useCmd switches the active namespace in the session file.
var useCmd = &cobra.Command{
	Use:   "use <namespace>",
	Short: "Switch the active namespace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		sess, err := requireSession()
		if err != nil {
			return err
		}

		sess.Namespace = name
		if err := saveSession(sess); err != nil {
			return fmt.Errorf("save session: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Active namespace set to %q.\n", name)
		return nil
	},
}

// contextCmd shows the current session context.
var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Show the current session context (server + namespace)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		sess, err := loadSession()
		if err != nil {
			return fmt.Errorf("load session: %w", err)
		}
		if sess == nil {
			fmt.Fprintln(cmd.OutOrStdout(), "Not logged in.")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Server:    %s\n", sess.Addr)
		if sess.Namespace != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Namespace: %s\n", sess.Namespace)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Namespace: (none)")
		}
		return nil
	},
}

// requireSession loads the session or returns an actionable error.
func requireSession() (*Session, error) {
	sess, err := loadSession()
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	if sess == nil {
		return nil, fmt.Errorf("not logged in — run: rcs login <addr>")
	}
	return sess, nil
}

// authedRequest makes an authenticated HTTP request to the RCS server using
// the session token. Namespace management endpoints don't require a namespace
// header, so none is sent here.
func authedRequest(ctx context.Context, sess *Session, method, path string, body []byte) (*http.Response, error) {
	addr := sess.Addr
	if len(addr) > 0 && addr[len(addr)-1] == '/' {
		addr = addr[:len(addr)-1]
	}
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, addr+path, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, addr+path, nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return http.DefaultClient.Do(req)
}

func init() {
	nsCmd.AddCommand(nsCreateCmd)
	nsCmd.AddCommand(nsListCmd)
	nsCmd.AddCommand(nsDeleteCmd)
	rootCmd.AddCommand(nsCmd)
	rootCmd.AddCommand(useCmd)
	rootCmd.AddCommand(contextCmd)
}
