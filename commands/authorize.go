package commands

import (
	"fmt"
	"os"

	"github.com/oskar1233/ksef/internal/settings"
)

func Authorize(filePath string, verifyCertificateChain *bool) error {
	cfg, client, err := loadSettingsAndClient()
	if err != nil {
		return err
	}
	if filePath == "" {
		filePath = cfg.SignedAuthRequestFile
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", filePath, err)
	}
	defer file.Close()

	verify := cfg.VerifyCertificateChain
	if verifyCertificateChain != nil {
		verify = *verifyCertificateChain
	}

	response, err := client.Authorize(file, verify)
	if err != nil {
		return fmt.Errorf("authorize: %w", err)
	}

	cfg.SignedAuthRequestFile = filePath
	cfg.AuthOperation = response
	cfg.AuthStatus = nil
	if err := settings.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("reference number: %s\nauthentication token valid until: %s\n", response.ReferenceNumber, response.AuthenticationToken.ValidUntil)
	fmt.Println("run `ksef get-auth-status --wait` and then `ksef redeem`")
	return nil
}
