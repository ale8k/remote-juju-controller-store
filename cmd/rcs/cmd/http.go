package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	rcsclient "github.com/ale8k/remote-juju-controller-store/pkg/client"
)

func normalizeAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	addr = strings.TrimRight(addr, "/")
	return addr
}

func authedAPIClient(s *Session) (rcsclient.ClientWithResponsesInterface, error) {
	return rcsclient.NewClientWithResponses(
		normalizeAddr(s.Addr),
		rcsclient.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+s.Token)
			if strings.TrimSpace(s.Namespace) != "" {
				req.Header.Set("X-RCS-Namespace", s.Namespace)
			}
			return nil
		}),
	)
}

func apiClient(addr string) (rcsclient.ClientWithResponsesInterface, error) {
	return rcsclient.NewClientWithResponses(normalizeAddr(addr))
}

func mustStatus(resp *http.Response, expected ...int) error {
	for _, code := range expected {
		if resp.StatusCode == code {
			return nil
		}
	}
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("unexpected status %s: %s", resp.Status, strings.TrimSpace(string(b)))
}
