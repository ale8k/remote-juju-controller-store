package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	rcsclient "github.com/ale8k/remote-juju-controller-store/pkg/client"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var loginToken string

var loginCmd = &cobra.Command{
	Use:   "login <rcs-url>",
	Short: "Login and create local RCS session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := normalizeAddr(args[0])
		tok := strings.TrimSpace(loginToken)
		if tok == "" {
			provider, err := getProviderInfo(cmd.Context(), addr)
			if err != nil {
				return err
			}
			tok, err = runDeviceFlow(provider)
			if err != nil {
				return err
			}
		}

		rcsTok, err := exchangeDeviceToken(cmd.Context(), addr, tok)
		if err != nil {
			return err
		}

		prev, _ := loadSession()
		ns := ""
		if prev != nil {
			ns = prev.Namespace
		}
		if err := saveSession(Session{Token: rcsTok, Addr: addr, Namespace: ns}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "logged in to %s\n", addr)
		return nil
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginToken, "token", "", "pre-obtained OIDC access token (skip device flow)")
}

type oidcDiscovery struct {
	DeviceAuthEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint      string `json:"token_endpoint"`
}

func getProviderInfo(ctx context.Context, addr string) (*rcsclient.AuthProviderResponse, error) {
	api, err := apiClient(addr)
	if err != nil {
		return nil, err
	}
	resp, err := api.GetAuthProviderWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected status %s: %s", resp.Status(), strings.TrimSpace(string(resp.Body)))
	}
	p := *resp.JSON200
	if p.Issuer == "" || p.ClientId == "" {
		return nil, fmt.Errorf("provider response missing issuer/client_id")
	}
	return &p, nil
}

func runDeviceFlow(provider *rcsclient.AuthProviderResponse) (string, error) {
	discoveryURL := strings.TrimRight(provider.Issuer, "/") + "/.well-known/openid-configuration"
	resp, err := http.Get(discoveryURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := mustStatus(resp, http.StatusOK); err != nil {
		return "", err
	}
	var disc oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
		return "", err
	}
	if disc.DeviceAuthEndpoint == "" || disc.TokenEndpoint == "" {
		return "", fmt.Errorf("issuer does not expose device flow endpoints")
	}

	cfg := &oauth2.Config{ClientID: provider.ClientId, Endpoint: oauth2.Endpoint{DeviceAuthURL: disc.DeviceAuthEndpoint, TokenURL: disc.TokenEndpoint}, Scopes: []string{"openid", "profile", "email"}}
	ctx := context.Background()
	respDevice, err := cfg.DeviceAuth(ctx)
	if err != nil {
		return "", err
	}

	fmt.Printf("Open: %s\n", respDevice.VerificationURI)
	if respDevice.VerificationURIComplete != "" {
		fmt.Printf("Or open: %s\n", respDevice.VerificationURIComplete)
	}
	fmt.Printf("Enter code: %s\n", respDevice.UserCode)
	fmt.Printf("Waiting for authorization...\n")

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	tok, err := cfg.DeviceAccessToken(ctxTimeout, respDevice)
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

func exchangeDeviceToken(ctx context.Context, addr, accessToken string) (string, error) {
	api, err := apiClient(addr)
	if err != nil {
		return "", err
	}
	resp, err := api.ExchangeDeviceTokenWithResponse(ctx, rcsclient.DeviceTokenRequest{Token: accessToken})
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		return "", fmt.Errorf("unexpected status %s: %s", resp.Status(), strings.TrimSpace(string(resp.Body)))
	}
	if strings.TrimSpace(resp.JSON200.Token) == "" {
		return "", fmt.Errorf("missing session token in response")
	}
	return resp.JSON200.Token, nil
}
