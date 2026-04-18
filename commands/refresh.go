package commands

import (
	"fmt"
	"strings"
	"time"

	ksef "github.com/oskar1233/ksef/internal"
	"github.com/oskar1233/ksef/internal/settings"
)

func Refresh() error {
	cfg, client, err := loadSettingsAndClient()
	if err != nil {
		return err
	}

	now := time.Now()
	if ksef.TokenStillValid(cfg.RefreshToken, now) {
		response, err := client.RefreshAccessToken(cfg.RefreshToken.Token)
		if err != nil {
			return fmt.Errorf("refresh access token: %w", err)
		}
		cfg.AccessToken = &response.AccessToken
		if err := settings.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("access token valid until: %s\n", response.AccessToken.ValidUntil)
		return nil
	}

	if cfg.KSeFToken != nil && strings.TrimSpace(cfg.KSeFToken.Token) != "" {
		fmt.Println("refresh token is missing or expired; starting KSeF token auth flow")
		if err := tokenAuthFlow(cfg, client); err != nil {
			return err
		}
		fmt.Printf("access token valid until: %s\nrefresh token valid until: %s\n", cfg.AccessToken.ValidUntil, cfg.RefreshToken.ValidUntil)
		return nil
	}

	return fmt.Errorf("refresh token is missing or expired and no KSeF token is saved; repeat XAdES auth and run `ksef generate-token`")
}
