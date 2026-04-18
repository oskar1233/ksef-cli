package commands

import (
	"fmt"
	"os"
	"time"

	ksef "github.com/oskar1233/ksef/internal"
	"github.com/oskar1233/ksef/internal/settings"
)

func Challenge(outputFile string) error {
	cfg, client, err := loadSettingsAndClient()
	if err != nil {
		return err
	}
	if err := ensureNIP(cfg); err != nil {
		return err
	}
	if outputFile == "" {
		outputFile = cfg.AuthRequestFile
	}

	challenge, err := client.Challenge()
	if err != nil {
		return fmt.Errorf("auth challenge: %w", err)
	}
	cfg.Challenge = challenge
	if err := settings.Save(cfg); err != nil {
		return err
	}

	xmlContent, err := ksef.BuildAuthTokenRequestXML(challenge.Challenge, cfg.NIP, cfg.SubjectIdentifierType)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputFile, []byte(xmlContent), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outputFile, err)
	}

	cfg.AuthRequest = &settings.AuthRequestState{
		File:        outputFile,
		GeneratedAt: time.Now().Format(time.RFC3339Nano),
	}
	cfg.AuthRequestFile = outputFile
	if err := settings.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("created %s for signing\n", outputFile)
	fmt.Println("sign it and run `ksef authorize --file <signed-xml>`")
	return nil
}
