package commands

import "fmt"

func Redeem(authenticationToken string) error {
	cfg, client, err := loadSettingsAndClient()
	if err != nil {
		return err
	}

	if authenticationToken == "" {
		_, authenticationToken = resolveAuthOperation(cfg)
	}
	if authenticationToken == "" {
		return fmt.Errorf("missing authentication token; run `ksef authorize` first")
	}

	response, err := client.AuthTokenRedeem(authenticationToken)
	if err != nil {
		return fmt.Errorf("redeem auth token: %w", err)
	}
	if err := saveAccessAndRefreshTokens(cfg, response); err != nil {
		return err
	}

	fmt.Printf("access token valid until: %s\nrefresh token valid until: %s\n", response.AccessToken.ValidUntil, response.RefreshToken.ValidUntil)
	fmt.Println("run `ksef generate-token` once to create a reusable KSeF token")
	return nil
}
