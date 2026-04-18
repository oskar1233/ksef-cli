package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	ksef "github.com/oskar1233/ksef/internal"
)

const (
	defaultEnvironment           = "demo"
	defaultSubjectIdentifierType = "certificateSubject"
	defaultAuthRequestFile       = "./auth_request.xml"
	defaultSignedAuthRequestFile = "./signed_auth_request.xml"
	defaultDownloadDir           = "./invoices"
	defaultPDFDir                = "./invoice-pdfs"
	defaultExportDir             = "./exports"
	defaultTokenDescription      = "ksef-cli"
)

var defaultTokenPermissions = []string{"InvoiceRead", "InvoiceWrite"}

type Settings struct {
	NIP                    string   `json:"nip"`
	Environment            string   `json:"environment"`
	BaseURL                string   `json:"base_url"`
	SubjectIdentifierType  string   `json:"subject_identifier_type"`
	VerifyCertificateChain bool     `json:"verify_certificate_chain"`
	AuthRequestFile        string   `json:"auth_request_file"`
	SignedAuthRequestFile  string   `json:"signed_auth_request_file"`
	DownloadDir            string   `json:"download_dir"`
	PDFDir                 string   `json:"pdf_dir"`
	ExportDir              string   `json:"export_dir"`
	TokenDescription       string   `json:"token_description"`
	TokenPermissions       []string `json:"token_permissions"`

	Challenge                    *ksef.ChallengeResponse                     `json:"challenge,omitempty"`
	AuthRequest                  *AuthRequestState                           `json:"auth_request,omitempty"`
	AuthOperation                *ksef.AuthenticationInitResponse            `json:"auth_operation,omitempty"`
	AuthStatus                   *ksef.AuthenticationOperationStatusResponse `json:"auth_status,omitempty"`
	AccessToken                  *ksef.TokenInfo                             `json:"access_token,omitempty"`
	RefreshToken                 *ksef.TokenInfo                             `json:"refresh_token,omitempty"`
	KSeFToken                    *KSeFTokenState                             `json:"ksef_token,omitempty"`
	TokenAuthChallenge           *ksef.ChallengeResponse                     `json:"token_auth_challenge,omitempty"`
	PublicKeyCertificates        []ksef.PublicKeyCertificate                 `json:"public_key_certificates,omitempty"`
	SelectedPublicKeyCertificate *ksef.PublicKeyCertificate                  `json:"selected_public_key_certificate,omitempty"`
	TokenAuthRequest             *ksef.InitTokenAuthenticationRequest        `json:"token_auth_request,omitempty"`
	TokenAuthOperation           *ksef.AuthenticationInitResponse            `json:"token_auth_operation,omitempty"`
	TokenAuthStatus              *ksef.AuthenticationOperationStatusResponse `json:"token_auth_status,omitempty"`
	LastInvoiceQuery             *InvoiceQueryState                          `json:"last_invoice_query,omitempty"`
	LastInvoiceDownload          *InvoiceDownloadState                       `json:"last_invoice_download,omitempty"`
	UpdatedAt                    string                                      `json:"updated_at"`
}

type AuthRequestState struct {
	File        string `json:"file"`
	GeneratedAt string `json:"generated_at"`
}

type KSeFTokenState struct {
	ReferenceNumber string   `json:"reference_number"`
	Token           string   `json:"token"`
	Status          string   `json:"status"`
	Description     string   `json:"description"`
	Permissions     []string `json:"permissions"`
	LastCheckedAt   string   `json:"last_checked_at,omitempty"`
}

type InvoiceQueryState struct {
	Month       string                              `json:"month"`
	SubjectType string                              `json:"subject_type"`
	DateType    string                              `json:"date_type"`
	From        string                              `json:"from"`
	To          string                              `json:"to"`
	SortOrder   string                              `json:"sort_order"`
	PageOffset  int                                 `json:"page_offset"`
	PageSize    int                                 `json:"page_size"`
	Response    *ksef.QueryInvoicesMetadataResponse `json:"response,omitempty"`
}

type InvoiceDownloadState struct {
	Month        string `json:"month"`
	Directory    string `json:"directory"`
	KSeFNumber   string `json:"ksef_number"`
	File         string `json:"file"`
	DownloadedAt string `json:"downloaded_at"`
}

func Path() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}

	return filepath.Join(homeDir, ".ksef", "settings.json"), nil
}

func Ensure() (*Settings, error) {
	settingsPath, err := Path()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating settings directory: %w", err)
	}

	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		cfg := defaults()
		if err := Save(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat settings file: %w", err)
	}

	cfg, err := Load()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func DefaultSettings() *Settings {
	return defaults()
}

func Load() (*Settings, error) {
	settingsPath, err := Path()
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("reading settings file: %w", err)
	}

	cfg := defaults()
	if len(strings.TrimSpace(string(content))) != 0 {
		if err := json.Unmarshal(content, cfg); err != nil {
			return nil, fmt.Errorf("unmarshal settings json: %w", err)
		}
	}

	before, _ := json.Marshal(cfg)
	cfg.normalize()
	after, _ := json.Marshal(cfg)
	if string(before) != string(after) {
		if err := Save(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func Save(cfg *Settings) error {
	cfg.normalize()
	cfg.UpdatedAt = time.Now().Format(time.RFC3339Nano)

	settingsPath, err := Path()
	if err != nil {
		return err
	}

	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings json: %w", err)
	}

	if err := os.WriteFile(settingsPath, append(content, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing settings file: %w", err)
	}

	return nil
}

func defaults() *Settings {
	cfg := &Settings{
		Environment:           defaultEnvironment,
		SubjectIdentifierType: defaultSubjectIdentifierType,
		AuthRequestFile:       defaultAuthRequestFile,
		SignedAuthRequestFile: defaultSignedAuthRequestFile,
		DownloadDir:           defaultDownloadDir,
		PDFDir:                defaultPDFDir,
		ExportDir:             defaultExportDir,
		TokenDescription:      defaultTokenDescription,
		TokenPermissions:      append([]string(nil), defaultTokenPermissions...),
	}
	cfg.BaseURL = defaultBaseURL(cfg.Environment)
	return cfg
}

func (cfg *Settings) normalize() {
	if cfg.Environment == "" {
		cfg.Environment = defaultEnvironment
	}
	cfg.Environment = strings.ToLower(cfg.Environment)

	if cfg.SubjectIdentifierType == "" {
		cfg.SubjectIdentifierType = defaultSubjectIdentifierType
	}
	if cfg.AuthRequestFile == "" {
		cfg.AuthRequestFile = defaultAuthRequestFile
	}
	if cfg.SignedAuthRequestFile == "" {
		cfg.SignedAuthRequestFile = defaultSignedAuthRequestFile
	}
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = defaultDownloadDir
	}
	if cfg.PDFDir == "" {
		cfg.PDFDir = defaultPDFDir
	}
	if cfg.ExportDir == "" {
		cfg.ExportDir = defaultExportDir
	}
	if cfg.TokenDescription == "" {
		cfg.TokenDescription = defaultTokenDescription
	}
	if len(cfg.TokenPermissions) == 0 {
		cfg.TokenPermissions = append([]string(nil), defaultTokenPermissions...)
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL(cfg.Environment)
	}
	if cfg.KSeFToken != nil && len(cfg.KSeFToken.Permissions) == 0 {
		cfg.KSeFToken.Permissions = append([]string(nil), cfg.TokenPermissions...)
	}
	if cfg.TokenPermissions != nil {
		cfg.TokenPermissions = slices.Compact(cfg.TokenPermissions)
	}
}

func defaultBaseURL(environment string) string {
	switch strings.ToLower(environment) {
	case "production", "prod":
		return "https://api.ksef.mf.gov.pl/v2"
	case "test":
		return "https://api-test.ksef.mf.gov.pl/v2"
	case "demo", "":
		return "https://api-demo.ksef.mf.gov.pl/v2"
	default:
		return "https://api-demo.ksef.mf.gov.pl/v2"
	}
}
