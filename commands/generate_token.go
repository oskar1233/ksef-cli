package commands

import (
	"fmt"
	"slices"
	"strings"
	"time"

	ksef "github.com/oskar1233/ksef/internal"
	"github.com/oskar1233/ksef/internal/settings"
)

func GenerateToken(description string, permissions []string, force bool) error {
	cfg, client, err := loadSettingsAndClient()
	if err != nil {
		return err
	}

	if cfg.KSeFToken != nil && strings.TrimSpace(cfg.KSeFToken.Token) != "" && !force {
		fmt.Println("KSeF token already exists in settings. Use --force to create a new one.")
		return nil
	}

	accessToken, err := ensureAccessToken(cfg, client)
	if err != nil {
		return err
	}

	if description == "" {
		description = cfg.TokenDescription
	}
	if len(permissions) == 0 {
		permissions = append([]string(nil), cfg.TokenPermissions...)
	}
	permissions = slices.Compact(permissions)

	response, err := client.GenerateToken(accessToken.Token, ksef.GenerateTokenRequest{
		Permissions: permissions,
		Description: description,
	})
	if err != nil {
		return fmt.Errorf("generate KSeF token: %w", err)
	}

	cfg.KSeFToken = &settings.KSeFTokenState{
		ReferenceNumber: response.ReferenceNumber,
		Token:           response.Token,
		Description:     description,
		Permissions:     permissions,
		Status:          "Pending",
	}
	if err := settings.Save(cfg); err != nil {
		return err
	}

	status, err := waitForTokenActive(cfg, client, accessToken.Token, response.ReferenceNumber)
	if err != nil {
		return err
	}

	fmt.Printf("KSeF token generated. reference number: %s\nstatus: %s\n", response.ReferenceNumber, status.Status)
	fmt.Println("back it up now; KSeF shows it only once and the CLI stores it in ~/.ksef/settings.json")
	return nil
}

func waitForTokenActive(cfg *settings.Settings, client ksef.API, accessToken string, referenceNumber string) (*ksef.TokenStatusResponse, error) {
	deadline := time.Now().Add(60 * time.Second)
	backoff := time.Second

	for {
		status, err := client.GetTokenStatus(accessToken, referenceNumber)
		if err != nil {
			return nil, fmt.Errorf("get KSeF token status: %w", err)
		}

		cfg.KSeFToken = &settings.KSeFTokenState{
			ReferenceNumber: status.ReferenceNumber,
			Token:           cfg.KSeFToken.Token,
			Description:     status.Description,
			Permissions:     status.RequestedPermissions,
			Status:          status.Status,
			LastCheckedAt:   time.Now().Format(time.RFC3339Nano),
		}
		if err := settings.Save(cfg); err != nil {
			return nil, err
		}

		switch status.Status {
		case "Active":
			return status, nil
		case "Failed", "Revoked", "Revoking":
			return status, fmt.Errorf("KSeF token status is %s: %s", status.Status, strings.Join(status.StatusDetails, "; "))
		}

		if time.Now().After(deadline) {
			return status, fmt.Errorf("timed out waiting for KSeF token to become active")
		}

		time.Sleep(backoff)
		if backoff < 8*time.Second {
			backoff *= 2
		}
	}
}
