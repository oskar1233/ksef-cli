package commands

import (
	"fmt"

	"github.com/oskar1233/ksef/internal/settings"
)

type InitOptions struct {
	NIP                    *string
	Environment            *string
	BaseURL                *string
	SubjectIdentifierType  *string
	VerifyCertificateChain *bool
	AuthRequestFile        *string
	SignedAuthRequestFile  *string
	DownloadDir            *string
	PDFDir                 *string
	ExportDir              *string
}

func Init(options InitOptions) error {
	cfg, err := settings.Ensure()
	if err != nil {
		return err
	}

	if options.NIP != nil {
		cfg.NIP = *options.NIP
	}
	if options.Environment != nil {
		cfg.Environment = *options.Environment
		if options.BaseURL == nil {
			cfg.BaseURL = ""
		}
	}
	if options.BaseURL != nil {
		cfg.BaseURL = *options.BaseURL
	}
	if options.SubjectIdentifierType != nil {
		cfg.SubjectIdentifierType = *options.SubjectIdentifierType
	}
	if options.VerifyCertificateChain != nil {
		cfg.VerifyCertificateChain = *options.VerifyCertificateChain
	}
	if options.AuthRequestFile != nil {
		cfg.AuthRequestFile = *options.AuthRequestFile
	}
	if options.SignedAuthRequestFile != nil {
		cfg.SignedAuthRequestFile = *options.SignedAuthRequestFile
	}
	if options.DownloadDir != nil {
		cfg.DownloadDir = *options.DownloadDir
	}
	if options.PDFDir != nil {
		cfg.PDFDir = *options.PDFDir
	}
	if options.ExportDir != nil {
		cfg.ExportDir = *options.ExportDir
	}

	if err := settings.Save(cfg); err != nil {
		return err
	}

	settingsPath, err := settings.Path()
	if err != nil {
		return err
	}
	fmt.Printf("saved %s\n", settingsPath)
	return nil
}
