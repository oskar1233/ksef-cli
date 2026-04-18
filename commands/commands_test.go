package commands

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	ksef "github.com/oskar1233/ksef/internal"
	"github.com/oskar1233/ksef/internal/mocks"
	"github.com/oskar1233/ksef/internal/render"
	"github.com/oskar1233/ksef/internal/settings"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestChallengeSavesStateAndWritesXML(t *testing.T) {
	prepareTestHome(t)
	setTestSettings(t, func(cfg *settings.Settings) {
		cfg.NIP = "1234567890"
	})

	api := &mocks.API{}
	useMockClient(t, api)

	api.On("Challenge").Return(&ksef.ChallengeResponse{
		Challenge:   "challenge-123",
		Timestamp:   time.Now().Format(time.RFC3339),
		TimestampMs: time.Now().UnixMilli(),
	}, nil).Once()

	outputFile := filepath.Join(t.TempDir(), "auth_request.xml")
	err := Challenge(outputFile)
	require.NoError(t, err)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	require.Contains(t, string(content), "challenge-123")
	require.Contains(t, string(content), "1234567890")

	cfg := loadSettings(t)
	require.NotNil(t, cfg.Challenge)
	require.Equal(t, "challenge-123", cfg.Challenge.Challenge)
	require.NotNil(t, cfg.AuthRequest)
	require.Equal(t, outputFile, cfg.AuthRequest.File)

	api.AssertExpectations(t)
}

func TestRedeemUsesSavedAuthOperationAndPersistsTokens(t *testing.T) {
	prepareTestHome(t)
	setTestSettings(t, func(cfg *settings.Settings) {
		cfg.AuthOperation = &ksef.AuthenticationInitResponse{
			ReferenceNumber: "ref-1",
			AuthenticationToken: ksef.TokenInfo{
				Token:      "auth-token",
				ValidUntil: time.Now().Add(time.Hour).Format(time.RFC3339),
			},
		}
	})

	api := &mocks.API{}
	useMockClient(t, api)
	api.On("AuthTokenRedeem", "auth-token").Return(&ksef.AuthenticationTokensResponse{
		AccessToken:  ksef.TokenInfo{Token: "access-1", ValidUntil: time.Now().Add(time.Hour).Format(time.RFC3339)},
		RefreshToken: ksef.TokenInfo{Token: "refresh-1", ValidUntil: time.Now().Add(24 * time.Hour).Format(time.RFC3339)},
	}, nil).Once()

	err := Redeem("")
	require.NoError(t, err)

	cfg := loadSettings(t)
	require.NotNil(t, cfg.AccessToken)
	require.Equal(t, "access-1", cfg.AccessToken.Token)
	require.NotNil(t, cfg.RefreshToken)
	require.Equal(t, "refresh-1", cfg.RefreshToken.Token)

	api.AssertExpectations(t)
}

func TestGenerateTokenSavesTokenState(t *testing.T) {
	prepareTestHome(t)
	setTestSettings(t, func(cfg *settings.Settings) {
		cfg.AccessToken = &ksef.TokenInfo{Token: "access-1", ValidUntil: time.Now().Add(time.Hour).Format(time.RFC3339)}
	})

	api := &mocks.API{}
	useMockClient(t, api)
	api.On("GenerateToken", "access-1", ksef.GenerateTokenRequest{
		Permissions: []string{"InvoiceRead", "InvoiceWrite"},
		Description: "ksef-cli",
	}).Return(&ksef.GenerateTokenResponse{ReferenceNumber: "token-ref-1", Token: "ksef-token-1"}, nil).Once()
	api.On("GetTokenStatus", "access-1", "token-ref-1").Return(&ksef.TokenStatusResponse{
		ReferenceNumber:      "token-ref-1",
		Description:          "ksef-cli",
		RequestedPermissions: []string{"InvoiceRead", "InvoiceWrite"},
		Status:               "Active",
	}, nil).Once()

	err := GenerateToken("", nil, false)
	require.NoError(t, err)

	cfg := loadSettings(t)
	require.NotNil(t, cfg.KSeFToken)
	require.Equal(t, "token-ref-1", cfg.KSeFToken.ReferenceNumber)
	require.Equal(t, "ksef-token-1", cfg.KSeFToken.Token)
	require.Equal(t, "Active", cfg.KSeFToken.Status)

	api.AssertExpectations(t)
}

func TestRefreshFallsBackToKSeFTokenFlow(t *testing.T) {
	prepareTestHome(t)
	setTestSettings(t, func(cfg *settings.Settings) {
		cfg.NIP = "1234567890"
		cfg.RefreshToken = &ksef.TokenInfo{Token: "expired-refresh", ValidUntil: time.Now().Add(-time.Hour).Format(time.RFC3339)}
		cfg.KSeFToken = &settings.KSeFTokenState{Token: "saved-ksef-token", ReferenceNumber: "token-ref-1", Status: "Active"}
	})

	certificate := testPublicKeyCertificate(t)
	api := &mocks.API{}
	useMockClient(t, api)

	api.On("Challenge").Return(&ksef.ChallengeResponse{
		Challenge:   "challenge-1",
		Timestamp:   time.Now().Format(time.RFC3339),
		TimestampMs: 1712862000000,
	}, nil).Once()
	api.On("GetPublicKeyCertificates").Return([]ksef.PublicKeyCertificate{certificate}, nil).Once()
	api.On("AuthenticateWithKSeFToken", mock.MatchedBy(func(request ksef.InitTokenAuthenticationRequest) bool {
		return request.Challenge == "challenge-1" && request.ContextIdentifier.Type == "Nip" && request.ContextIdentifier.Value == "1234567890" && request.EncryptedToken != ""
	})).Return(&ksef.AuthenticationInitResponse{
		ReferenceNumber:     "auth-ref-1",
		AuthenticationToken: ksef.TokenInfo{Token: "auth-token-1", ValidUntil: time.Now().Add(time.Hour).Format(time.RFC3339)},
	}, nil).Once()
	api.On("AuthStatus", "auth-ref-1", "auth-token-1").Return(&ksef.AuthenticationOperationStatusResponse{
		Status: ksef.StatusInfo{Code: 200, Description: "OK"},
	}, nil).Once()
	api.On("AuthTokenRedeem", "auth-token-1").Return(&ksef.AuthenticationTokensResponse{
		AccessToken:  ksef.TokenInfo{Token: "access-2", ValidUntil: time.Now().Add(time.Hour).Format(time.RFC3339)},
		RefreshToken: ksef.TokenInfo{Token: "refresh-2", ValidUntil: time.Now().Add(24 * time.Hour).Format(time.RFC3339)},
	}, nil).Once()

	err := Refresh()
	require.NoError(t, err)

	cfg := loadSettings(t)
	require.NotNil(t, cfg.TokenAuthRequest)
	require.NotEmpty(t, cfg.TokenAuthRequest.EncryptedToken)
	require.Equal(t, "access-2", cfg.AccessToken.Token)
	require.Equal(t, "refresh-2", cfg.RefreshToken.Token)

	api.AssertExpectations(t)
}

func TestExportCSVWritesSeparateFiles(t *testing.T) {
	prepareTestHome(t)
	setTestSettings(t, func(cfg *settings.Settings) {
		cfg.AccessToken = &ksef.TokenInfo{Token: "access-1", ValidUntil: time.Now().Add(time.Hour).Format(time.RFC3339)}
	})

	api := &mocks.API{}
	useMockClient(t, api)
	api.On("QueryInvoicesMetadata", "access-1", mock.MatchedBy(func(filters ksef.InvoiceQueryFilters) bool {
		return filters.SubjectType == "Subject2"
	}), "Asc", 0, 250).Return(&ksef.QueryInvoicesMetadataResponse{
		HasMore:  false,
		Invoices: []ksef.InvoiceMetadata{{KSeFNumber: "purchase-1", InvoiceNumber: "P/1", IssueDate: "2026-04-02", Seller: ksef.InvoiceMetadataSeller{Name: "Seller P"}, Currency: "PLN"}},
	}, nil).Once()
	api.On("QueryInvoicesMetadata", "access-1", mock.MatchedBy(func(filters ksef.InvoiceQueryFilters) bool {
		return filters.SubjectType == "Subject1"
	}), "Asc", 0, 250).Return(&ksef.QueryInvoicesMetadataResponse{
		HasMore:  false,
		Invoices: []ksef.InvoiceMetadata{{KSeFNumber: "sales-1", InvoiceNumber: "S/1", IssueDate: "2026-04-03", Seller: ksef.InvoiceMetadataSeller{Name: "Seller S"}, Currency: "PLN"}},
	}, nil).Once()

	outputDir := t.TempDir()
	err := ExportCSV("2026-04", outputDir, "both")
	require.NoError(t, err)

	purchaseCSV, err := os.ReadFile(filepath.Join(outputDir, "purchase_2026-04.csv"))
	require.NoError(t, err)
	require.Contains(t, string(purchaseCSV), "purchase-1")

	salesCSV, err := os.ReadFile(filepath.Join(outputDir, "sales_2026-04.csv"))
	require.NoError(t, err)
	require.Contains(t, string(salesCSV), "sales-1")

	api.AssertExpectations(t)
}

func TestDownloadPDFsRendersVisualizationFiles(t *testing.T) {
	prepareTestHome(t)
	setTestSettings(t, func(cfg *settings.Settings) {
		cfg.AccessToken = &ksef.TokenInfo{Token: "access-1", ValidUntil: time.Now().Add(time.Hour).Format(time.RFC3339)}
	})

	restore := render.SetPDFGeneratorForTesting(func(_ context.Context, htmlPath string, outputPath string) error {
		content, err := os.ReadFile(htmlPath)
		require.NoError(t, err)
		require.Contains(t, string(content), "P/1")
		return os.WriteFile(outputPath, []byte("%PDF-1.4\n%dummy\n"), 0o644)
	})
	defer restore()

	api := &mocks.API{}
	useMockClient(t, api)
	api.On("QueryInvoicesMetadata", "access-1", mock.MatchedBy(func(filters ksef.InvoiceQueryFilters) bool {
		return filters.SubjectType == "Subject2"
	}), "Asc", 0, 250).Return(&ksef.QueryInvoicesMetadataResponse{
		HasMore: false,
		Invoices: []ksef.InvoiceMetadata{{
			KSeFNumber:    "purchase-1",
			InvoiceNumber: "P/1",
			IssueDate:     "2026-04-02",
			Seller:        ksef.InvoiceMetadataSeller{Name: "Seller P", NIP: "123"},
			Buyer:         ksef.InvoiceMetadataBuyer{Name: "Buyer P", Identifier: ksef.AuthenticationContextIdentifier{Type: "Nip", Value: "999"}},
			Currency:      "PLN",
		}},
	}, nil).Once()
	api.On("DownloadInvoice", "access-1", "purchase-1").Return([]byte(`<Faktura><Fa><P_2>P/1</P_2><KodWaluty>PLN</KodWaluty></Fa></Faktura>`), http.Header{}, nil).Once()

	outputDir := t.TempDir()
	err := DownloadPDFs("2026-04", outputDir, "purchase", false, false)
	require.NoError(t, err)

	files, err := filepath.Glob(filepath.Join(outputDir, "2026-04", "purchase", "*.pdf"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	info, err := os.Stat(files[0])
	require.NoError(t, err)
	require.True(t, info.Size() > 0)

	api.AssertExpectations(t)
}

func TestDownloadPDFsKeepsIntermediateHTMLWhenRequested(t *testing.T) {
	prepareTestHome(t)
	setTestSettings(t, func(cfg *settings.Settings) {
		cfg.AccessToken = &ksef.TokenInfo{Token: "access-1", ValidUntil: time.Now().Add(time.Hour).Format(time.RFC3339)}
	})

	restore := render.SetPDFGeneratorForTesting(func(_ context.Context, htmlPath string, outputPath string) error {
		return os.WriteFile(outputPath, []byte("%PDF-1.4\n%dummy\n"), 0o644)
	})
	defer restore()

	api := &mocks.API{}
	useMockClient(t, api)
	api.On("QueryInvoicesMetadata", "access-1", mock.MatchedBy(func(filters ksef.InvoiceQueryFilters) bool {
		return filters.SubjectType == "Subject2"
	}), "Asc", 0, 250).Return(&ksef.QueryInvoicesMetadataResponse{
		HasMore: false,
		Invoices: []ksef.InvoiceMetadata{{
			KSeFNumber:    "purchase-1",
			InvoiceNumber: "P/1",
			IssueDate:     "2026-04-02",
			Seller:        ksef.InvoiceMetadataSeller{Name: "Seller P", NIP: "123"},
			Buyer:         ksef.InvoiceMetadataBuyer{Name: "Buyer P", Identifier: ksef.AuthenticationContextIdentifier{Type: "Nip", Value: "999"}},
			Currency:      "PLN",
		}},
	}, nil).Once()
	api.On("DownloadInvoice", "access-1", "purchase-1").Return([]byte(`<Faktura><Fa><P_2>P/1</P_2><KodWaluty>PLN</KodWaluty></Fa></Faktura>`), http.Header{}, nil).Once()

	outputDir := t.TempDir()
	err := DownloadPDFs("2026-04", outputDir, "purchase", false, true)
	require.NoError(t, err)

	htmlFiles, err := filepath.Glob(filepath.Join(outputDir, "2026-04", "purchase", "*.html"))
	require.NoError(t, err)
	require.Len(t, htmlFiles, 1)

	htmlContent, err := os.ReadFile(htmlFiles[0])
	require.NoError(t, err)
	require.Contains(t, string(htmlContent), "P/1")

	api.AssertExpectations(t)
}

func TestDownloadDownloadsInvoiceXMLs(t *testing.T) {
	prepareTestHome(t)
	setTestSettings(t, func(cfg *settings.Settings) {
		cfg.AccessToken = &ksef.TokenInfo{Token: "access-1", ValidUntil: time.Now().Add(time.Hour).Format(time.RFC3339)}
	})

	api := &mocks.API{}
	useMockClient(t, api)
	api.On("QueryInvoicesMetadata", "access-1", mock.AnythingOfType("ksef.InvoiceQueryFilters"), "Asc", 0, 250).Return(&ksef.QueryInvoicesMetadataResponse{
		HasMore:     false,
		IsTruncated: false,
		Invoices: []ksef.InvoiceMetadata{
			{
				KSeFNumber:    "1234567890-202604-01",
				InvoiceNumber: "FV/1/04/2026",
				IssueDate:     "2026-04-02",
				Seller:        ksef.InvoiceMetadataSeller{Name: "Demo Seller"},
				GrossAmount:   123.45,
				Currency:      "PLN",
			},
		},
	}, nil).Once()
	api.On("DownloadInvoice", "access-1", "1234567890-202604-01").Return([]byte("<xml>invoice</xml>"), http.Header{}, nil).Once()

	outputDir := t.TempDir()
	err := Download("2026-04", outputDir)
	require.NoError(t, err)

	files, err := filepath.Glob(filepath.Join(outputDir, "2026-04", "*.xml"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(files[0])
	require.NoError(t, err)
	require.Equal(t, "<xml>invoice</xml>", string(content))

	cfg := loadSettings(t)
	require.NotNil(t, cfg.LastInvoiceDownload)
	require.Equal(t, files[0], cfg.LastInvoiceDownload.File)

	api.AssertExpectations(t)
}

func prepareTestHome(t *testing.T) {
	t.Helper()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	_, err := settings.Ensure()
	require.NoError(t, err)
}

func setTestSettings(t *testing.T, mutate func(cfg *settings.Settings)) {
	t.Helper()
	cfg := loadSettings(t)
	mutate(cfg)
	require.NoError(t, settings.Save(cfg))
}

func loadSettings(t *testing.T) *settings.Settings {
	t.Helper()
	cfg, err := settings.Load()
	require.NoError(t, err)
	return cfg
}

func useMockClient(t *testing.T, api *mocks.API) {
	t.Helper()
	oldNewClient := newClient
	newClient = func(baseURL string) ksef.API {
		return api
	}
	t.Cleanup(func() {
		newClient = oldNewClient
	})
}

func testPublicKeyCertificate(t *testing.T) ksef.PublicKeyCertificate {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          bigIntOne(),
		Subject:               pkix.Name{CommonName: "KSeF test encryption cert"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	return ksef.PublicKeyCertificate{
		Certificate: base64.StdEncoding.EncodeToString(der),
		ValidFrom:   template.NotBefore.Format(time.RFC3339),
		ValidTo:     template.NotAfter.Format(time.RFC3339),
		Usage:       []string{"KsefTokenEncryption"},
	}
}

func bigIntOne() *big.Int {
	return big.NewInt(1)
}
