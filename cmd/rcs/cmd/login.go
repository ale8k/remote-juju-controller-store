package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ale8k/remote-juju-controller-store/pkg/client"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// oidcDiscovery holds the subset of fields from an OIDC discovery document we need.
type oidcDiscovery struct {
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
}

var loginCmd = &cobra.Command{
	Use:   "login <rcsd-addr>",
	Short: "Log in to a remote controller store",
	Long: `Authenticates with the remote controller store at the given address.
Performs an OIDC device flow login against the configured identity provider,
then exchanges the resulting token for an RCS session token.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := args[0]
		ctx := cmd.Context()

		// Check for an existing session first.
		existing, err := loadSession()
		if err != nil {
			return fmt.Errorf("load session: %w", err)
		}
		if existing != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Already logged in to %s. Run 'rcs logout' first.\n", existing.Addr)
			return nil
		}

		rcsClient, err := client.NewClientWithResponses(addr)
		if err != nil {
			return fmt.Errorf("create client: %w", err)
		}

		// 1. Get OIDC provider config from rcsd.
		provResp, err := rcsClient.GetAuthProviderWithResponse(ctx)
		if err != nil {
			return fmt.Errorf("fetch provider config: %w", err)
		}
		if provResp.StatusCode() != http.StatusOK || provResp.JSON200 == nil {
			return fmt.Errorf("fetch provider config: rcsd returned %d", provResp.StatusCode())
		}
		prov := provResp.JSON200

		// 2. OIDC discovery — find device auth + token endpoints.
		disc, err := discoverOIDC(ctx, prov.Issuer)
		if err != nil {
			return fmt.Errorf("OIDC discovery: %w", err)
		}

		// 3. Start device flow.
		oauth2Cfg := &oauth2.Config{
			ClientID: prov.ClientId,
			Endpoint: oauth2.Endpoint{
				DeviceAuthURL: disc.DeviceAuthorizationEndpoint,
				TokenURL:      disc.TokenEndpoint,
				AuthStyle:     oauth2.AuthStyleInParams,
			},
			Scopes: []string{"openid", "email", "profile"},
		}
		deviceResp, err := oauth2Cfg.DeviceAuth(ctx)
		if err != nil {
			return fmt.Errorf("start device flow: %w", err)
		}

		// 4. Prompt user.
		if deviceResp.VerificationURIComplete != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Open this URL to log in:\n\n  %s\n\nWaiting...\n", deviceResp.VerificationURIComplete)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Open this URL in your browser:\n\n  %s\n\nAnd enter code: %s\n\nWaiting...\n",
				deviceResp.VerificationURI, deviceResp.UserCode)
		}

		// 5. Poll until user completes the browser flow.
		keycloakToken, err := oauth2Cfg.DeviceAccessToken(ctx, deviceResp)
		if err != nil {
			return fmt.Errorf("waiting for login: %w", err)
		}

		// 6. Exchange the Keycloak access token for an RCS session token.
		exchResp, err := rcsClient.ExchangeDeviceTokenWithResponse(ctx, client.ExchangeDeviceTokenJSONRequestBody{
			Token: keycloakToken.AccessToken,
		})
		if err != nil {
			return fmt.Errorf("exchange token: %w", err)
		}
		if exchResp.StatusCode() != http.StatusOK || exchResp.JSON200 == nil {
			return fmt.Errorf("exchange token: rcsd returned %d: %s", exchResp.StatusCode(), string(exchResp.Body))
		}

		// 7. Persist session.
		if err := saveSession(&Session{Addr: addr, Token: exchResp.JSON200.Token}); err != nil {
			return fmt.Errorf("save session: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Logged in successfully.")
		return nil
	},
}

func discoverOIDC(ctx context.Context, issuer string) (*oidcDiscovery, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery returned %d", resp.StatusCode)
	}
	var d oidcDiscovery
	return &d, json.NewDecoder(resp.Body).Decode(&d)
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
