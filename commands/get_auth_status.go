package commands

import (
	"fmt"
	"time"

	"github.com/oskar1233/ksef/internal/settings"
)

func GetAuthStatus(authenticationToken string, referenceNumber string, wait bool, timeout time.Duration) error {
	cfg, client, err := loadSettingsAndClient()
	if err != nil {
		return err
	}

	if referenceNumber == "" || authenticationToken == "" {
		savedReference, savedToken := resolveAuthOperation(cfg)
		if referenceNumber == "" {
			referenceNumber = savedReference
		}
		if authenticationToken == "" {
			authenticationToken = savedToken
		}
	}
	if referenceNumber == "" || authenticationToken == "" {
		return fmt.Errorf("missing referenceNumber/authenticationToken; run `ksef authorize` first or pass flags explicitly")
	}

	if wait {
		status, err := waitForAuthStatus(client, referenceNumber, authenticationToken, timeout, cfg)
		if err != nil {
			return err
		}
		fmt.Printf("auth status: %d %s\n", status.Status.Code, status.Status.Description)
		return nil
	}

	status, err := client.AuthStatus(referenceNumber, authenticationToken)
	if err != nil {
		return fmt.Errorf("auth status: %w", err)
	}
	cfg.AuthStatus = status
	if cfg.TokenAuthOperation != nil && cfg.TokenAuthOperation.ReferenceNumber == referenceNumber {
		cfg.TokenAuthStatus = status
	}
	if err := settings.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("auth status: %d %s\n", status.Status.Code, status.Status.Description)
	for _, detail := range status.Status.Details {
		fmt.Printf("detail: %s\n", detail)
	}
	return nil
}

func resolveAuthOperation(cfg *settings.Settings) (string, string) {
	if cfg.AuthOperation != nil {
		return cfg.AuthOperation.ReferenceNumber, cfg.AuthOperation.AuthenticationToken.Token
	}
	if cfg.TokenAuthOperation != nil {
		return cfg.TokenAuthOperation.ReferenceNumber, cfg.TokenAuthOperation.AuthenticationToken.Token
	}
	return "", ""
}
