package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	ksef "github.com/oskar1233/ksef/internal"
	"github.com/oskar1233/ksef/internal/settings"
)

const (
	invoiceKindPurchase = "purchase"
	invoiceKindSales    = "sales"
	invoiceKindBoth     = "both"
)

var newClient = func(baseURL string) ksef.API {
	return ksef.NewClient(baseURL)
}

func loadSettingsAndClient() (*settings.Settings, ksef.API, error) {
	cfg, err := settings.Ensure()
	if err != nil {
		return nil, nil, err
	}

	return cfg, newClient(cfg.BaseURL), nil
}

func ensureNIP(cfg *settings.Settings) error {
	if strings.TrimSpace(cfg.NIP) == "" {
		return fmt.Errorf("settings.nip is empty; edit ~/.ksef/settings.json or run `ksef init --nip ...`")
	}
	return nil
}

func saveAccessAndRefreshTokens(cfg *settings.Settings, response *ksef.AuthenticationTokensResponse) error {
	cfg.AccessToken = &response.AccessToken
	cfg.RefreshToken = &response.RefreshToken
	return settings.Save(cfg)
}

func ensureAccessToken(cfg *settings.Settings, client ksef.API) (*ksef.TokenInfo, error) {
	now := time.Now()
	if ksef.TokenStillValid(cfg.AccessToken, now) {
		return cfg.AccessToken, nil
	}

	if ksef.TokenStillValid(cfg.RefreshToken, now) {
		response, err := client.RefreshAccessToken(cfg.RefreshToken.Token)
		if err != nil {
			return nil, fmt.Errorf("refresh access token: %w", err)
		}
		cfg.AccessToken = &response.AccessToken
		if err := settings.Save(cfg); err != nil {
			return nil, err
		}
		return cfg.AccessToken, nil
	}

	if cfg.KSeFToken != nil && strings.TrimSpace(cfg.KSeFToken.Token) != "" {
		if err := tokenAuthFlow(cfg, client); err != nil {
			return nil, err
		}
		if cfg.AccessToken == nil {
			return nil, fmt.Errorf("token auth finished without access token")
		}
		return cfg.AccessToken, nil
	}

	return nil, fmt.Errorf("no valid access token, refresh token, or KSeF token available; finish XAdES auth first")
}

func tokenAuthFlow(cfg *settings.Settings, client ksef.API) error {
	if err := ensureNIP(cfg); err != nil {
		return err
	}
	if cfg.KSeFToken == nil || strings.TrimSpace(cfg.KSeFToken.Token) == "" {
		return fmt.Errorf("settings.ksef_token is empty; run `ksef generate-token` first")
	}

	challenge, err := client.Challenge()
	if err != nil {
		return fmt.Errorf("auth challenge: %w", err)
	}
	cfg.TokenAuthChallenge = challenge
	if err := settings.Save(cfg); err != nil {
		return err
	}

	certificates, err := client.GetPublicKeyCertificates()
	if err != nil {
		return fmt.Errorf("public key certificates: %w", err)
	}
	cfg.PublicKeyCertificates = certificates
	if err := settings.Save(cfg); err != nil {
		return err
	}

	encryptedToken, selectedCertificate, err := ksef.EncryptKSeFToken(cfg.KSeFToken.Token, challenge.TimestampMs, certificates)
	if err != nil {
		return err
	}
	cfg.SelectedPublicKeyCertificate = selectedCertificate
	cfg.TokenAuthRequest = &ksef.InitTokenAuthenticationRequest{
		Challenge: challenge.Challenge,
		ContextIdentifier: ksef.AuthenticationContextIdentifier{
			Type:  "Nip",
			Value: cfg.NIP,
		},
		EncryptedToken: encryptedToken,
	}
	if err := settings.Save(cfg); err != nil {
		return err
	}

	operation, err := client.AuthenticateWithKSeFToken(*cfg.TokenAuthRequest)
	if err != nil {
		return fmt.Errorf("authenticate with KSeF token: %w", err)
	}
	cfg.TokenAuthOperation = operation
	cfg.TokenAuthStatus = nil
	if err := settings.Save(cfg); err != nil {
		return err
	}

	status, err := waitForAuthStatus(client, operation.ReferenceNumber, operation.AuthenticationToken.Token, 60*time.Second, cfg)
	if err != nil {
		return err
	}
	if status.Status.Code != 200 {
		return fmt.Errorf("token auth finished with status %d: %s", status.Status.Code, status.Status.Description)
	}

	tokens, err := client.AuthTokenRedeem(operation.AuthenticationToken.Token)
	if err != nil {
		return fmt.Errorf("redeem auth token: %w", err)
	}
	return saveAccessAndRefreshTokens(cfg, tokens)
}

func waitForAuthStatus(client ksef.API, referenceNumber string, authenticationToken string, timeout time.Duration, cfg *settings.Settings) (*ksef.AuthenticationOperationStatusResponse, error) {
	deadline := time.Now().Add(timeout)
	backoff := time.Second

	for {
		status, err := client.AuthStatus(referenceNumber, authenticationToken)
		if err != nil {
			return nil, fmt.Errorf("auth status: %w", err)
		}

		if cfg != nil {
			if cfg.TokenAuthOperation != nil && cfg.TokenAuthOperation.ReferenceNumber == referenceNumber {
				cfg.TokenAuthStatus = status
			} else {
				cfg.AuthStatus = status
			}
			if err := settings.Save(cfg); err != nil {
				return nil, err
			}
		}

		if status.Status.Code != 100 {
			return status, nil
		}
		if time.Now().After(deadline) {
			return status, fmt.Errorf("auth status polling timed out after %s", timeout)
		}

		time.Sleep(backoff)
		if backoff < 8*time.Second {
			backoff *= 2
		}
	}
}

func currentMonth() string {
	return time.Now().Format("2006-01")
}

func lastMonth() string {
	return time.Now().AddDate(0, -1, 0).Format("2006-01")
}

func monthRange(month string) (time.Time, time.Time, error) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("load Europe/Warsaw location: %w", err)
	}

	start, err := time.ParseInLocation("2006-01", month, location)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid month %q, expected YYYY-MM", month)
	}
	start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, location)
	end := start.AddDate(0, 1, 0).Add(-time.Second)

	now := time.Now().In(location)
	if now.Before(end) {
		end = now
	}

	return start, end, nil
}

func normalizeInvoiceKind(kind string, defaultValue string) (string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = defaultValue
	}
	switch kind {
	case invoiceKindPurchase, invoiceKindSales, invoiceKindBoth:
		return kind, nil
	default:
		return "", fmt.Errorf("invalid subject %q; use purchase, sales, or both", kind)
	}
}

func invoiceKinds(kind string, defaultValue string) ([]string, error) {
	normalized, err := normalizeInvoiceKind(kind, defaultValue)
	if err != nil {
		return nil, err
	}
	if normalized == invoiceKindBoth {
		return []string{invoiceKindPurchase, invoiceKindSales}, nil
	}
	return []string{normalized}, nil
}

func subjectTypeForKind(kind string) (string, string, error) {
	switch kind {
	case invoiceKindPurchase:
		return "Subject2", "purchase", nil
	case invoiceKindSales:
		return "Subject1", "sales", nil
	default:
		return "", "", fmt.Errorf("unsupported invoice kind %q", kind)
	}
}

func queryInvoicesForMonth(cfg *settings.Settings, client ksef.API, month string) ([]ksef.InvoiceMetadata, error) {
	return queryInvoicesForMonthByKind(cfg, client, month, invoiceKindPurchase)
}

func queryInvoicesForMonthByKind(cfg *settings.Settings, client ksef.API, month string, kind string) ([]ksef.InvoiceMetadata, error) {
	from, to, err := monthRange(month)
	if err != nil {
		return nil, err
	}

	subjectType, _, err := subjectTypeForKind(kind)
	if err != nil {
		return nil, err
	}

	pageSize := 250
	dateType := "PermanentStorage"
	sortOrder := "Asc"
	restrict := true

	collected := make([]ksef.InvoiceMetadata, 0)
	seen := make(map[string]struct{})
	currentFrom := from
	lastFrom := ""

	for {
		pageOffset := 0
		for {
			accessToken, err := ensureAccessToken(cfg, client)
			if err != nil {
				return nil, err
			}

			filters := ksef.InvoiceQueryFilters{
				SubjectType: subjectType,
				DateRange: ksef.InvoiceQueryDateRange{
					DateType:                          dateType,
					From:                              currentFrom.Format(time.RFC3339),
					To:                                to.Format(time.RFC3339),
					RestrictToPermanentStorageHwmDate: &restrict,
				},
			}

			response, err := client.QueryInvoicesMetadata(accessToken.Token, filters, sortOrder, pageOffset, pageSize)
			if err != nil {
				return nil, fmt.Errorf("query invoices metadata: %w", err)
			}

			cfg.LastInvoiceQuery = &settings.InvoiceQueryState{
				Month:       month,
				SubjectType: subjectType,
				DateType:    dateType,
				From:        filters.DateRange.From,
				To:          filters.DateRange.To,
				SortOrder:   sortOrder,
				PageOffset:  pageOffset,
				PageSize:    pageSize,
				Response:    response,
			}
			if err := settings.Save(cfg); err != nil {
				return nil, err
			}

			for _, invoice := range response.Invoices {
				if _, exists := seen[invoice.KSeFNumber]; exists {
					continue
				}
				seen[invoice.KSeFNumber] = struct{}{}
				collected = append(collected, invoice)
			}

			if !response.HasMore {
				return collected, nil
			}

			if response.IsTruncated {
				if len(response.Invoices) == 0 {
					return nil, fmt.Errorf("invoice query was truncated without any returned invoices")
				}

				lastInvoice := response.Invoices[len(response.Invoices)-1]
				nextFrom, err := time.Parse(time.RFC3339, lastInvoice.PermanentStorageDate)
				if err != nil {
					return nil, fmt.Errorf("parse permanent storage date %q: %w", lastInvoice.PermanentStorageDate, err)
				}
				nextFromValue := nextFrom.Format(time.RFC3339)
				if nextFromValue == lastFrom {
					return nil, fmt.Errorf("invoice query did not advance after truncated response at %s", nextFromValue)
				}
				lastFrom = nextFromValue
				currentFrom = nextFrom
				break
			}

			pageOffset++
		}
	}
}

func queryInvoicesForKinds(cfg *settings.Settings, client ksef.API, month string, kind string, defaultKind string) (map[string][]ksef.InvoiceMetadata, error) {
	kinds, err := invoiceKinds(kind, defaultKind)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]ksef.InvoiceMetadata, len(kinds))
	for _, currentKind := range kinds {
		invoices, err := queryInvoicesForMonthByKind(cfg, client, month, currentKind)
		if err != nil {
			return nil, err
		}
		result[currentKind] = invoices
	}
	return result, nil
}

func printInvoices(invoices []ksef.InvoiceMetadata, format string) error {
	switch format {
	case "json":
		content, err := json.MarshalIndent(invoices, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal invoices json: %w", err)
		}
		fmt.Println(string(content))
		return nil
	case "csv":
		writer := csv.NewWriter(os.Stdout)
		if err := writer.Write([]string{"ksef_number", "seller", "invoice_number", "issue_date", "gross", "currency"}); err != nil {
			return err
		}
		for _, invoice := range invoices {
			if err := writer.Write([]string{invoice.KSeFNumber, invoice.Seller.Name, invoice.InvoiceNumber, invoice.IssueDate, fmt.Sprintf("%.2f", invoice.GrossAmount), invoice.Currency}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	default:
		return printInvoiceTable(invoices)
	}
}

func printInvoicesGrouped(grouped map[string][]ksef.InvoiceMetadata, format string) error {
	if format == "json" {
		content, err := json.MarshalIndent(grouped, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal grouped invoices json: %w", err)
		}
		fmt.Println(string(content))
		return nil
	}

	orderedKinds := []string{invoiceKindPurchase, invoiceKindSales}
	for _, kind := range orderedKinds {
		invoices, exists := grouped[kind]
		if !exists {
			continue
		}
		fmt.Printf("\n%s invoices\n", strings.Title(kind))
		fmt.Println(strings.Repeat("=", len(kind)+9))
		if err := printInvoices(invoices, format); err != nil {
			return err
		}
	}
	return nil
}

func printInvoiceTable(invoices []ksef.InvoiceMetadata) error {
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "KSeF Number\tSeller\tInvoice #\tDate\tGross")
	totals := map[string]float64{}
	for _, invoice := range invoices {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%.2f %s\n", invoice.KSeFNumber, invoice.Seller.Name, invoice.InvoiceNumber, invoice.IssueDate, invoice.GrossAmount, invoice.Currency)
		totals[invoice.Currency] += invoice.GrossAmount
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	fmt.Printf("%d invoices\n", len(invoices))
	for currency, total := range totals {
		fmt.Printf("total %s: %.2f\n", currency, total)
	}
	return nil
}

func writeInvoicesCSVFile(path string, kind string, invoices []ksef.InvoiceMetadata) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create csv directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv file %s: %w", path, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write([]string{
		"kind",
		"ksef_number",
		"invoice_number",
		"issue_date",
		"invoicing_date",
		"acquisition_date",
		"permanent_storage_date",
		"seller_nip",
		"seller_name",
		"buyer_identifier_type",
		"buyer_identifier_value",
		"buyer_name",
		"net_amount",
		"gross_amount",
		"vat_amount_pln",
		"currency",
		"invoicing_mode",
		"invoice_type",
		"has_attachment",
	}); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for _, invoice := range invoices {
		if err := writer.Write([]string{
			kind,
			invoice.KSeFNumber,
			invoice.InvoiceNumber,
			invoice.IssueDate,
			invoice.InvoicingDate,
			invoice.AcquisitionDate,
			invoice.PermanentStorageDate,
			invoice.Seller.NIP,
			invoice.Seller.Name,
			invoice.Buyer.Identifier.Type,
			invoice.Buyer.Identifier.Value,
			invoice.Buyer.Name,
			fmt.Sprintf("%.2f", invoice.NetAmount),
			fmt.Sprintf("%.2f", invoice.GrossAmount),
			fmt.Sprintf("%.2f", invoice.VATAmount),
			invoice.Currency,
			invoice.InvoicingMode,
			invoice.InvoiceType,
			fmt.Sprintf("%t", invoice.HasAttachment),
		}); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv writer: %w", err)
	}
	return nil
}

func sanitizeFileNamePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}

	mapped := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		switch r {
		case '-', '_', '.':
			return r
		case ' ':
			return '_'
		default:
			return '_'
		}
	}, value)
	mapped = strings.Trim(mapped, "._")
	for strings.Contains(mapped, "__") {
		mapped = strings.ReplaceAll(mapped, "__", "_")
	}
	if mapped == "" {
		mapped = "unknown"
	}
	if len(mapped) > 64 {
		mapped = mapped[:64]
	}
	return mapped
}

func invoiceFilePath(baseDir string, month string, invoice ksef.InvoiceMetadata) string {
	fileName := fmt.Sprintf("%s_%s_%s_%s.xml",
		sanitizeFileNamePart(invoice.IssueDate),
		sanitizeFileNamePart(invoice.Seller.Name),
		sanitizeFileNamePart(invoice.InvoiceNumber),
		sanitizeFileNamePart(invoice.KSeFNumber),
	)
	return filepath.Join(baseDir, month, fileName)
}

func invoicePDFPath(baseDir string, month string, kind string, invoice ksef.InvoiceMetadata) string {
	fileName := fmt.Sprintf("%s_%s_%s_%s.pdf",
		sanitizeFileNamePart(invoice.IssueDate),
		sanitizeFileNamePart(invoice.Seller.Name),
		sanitizeFileNamePart(invoice.InvoiceNumber),
		sanitizeFileNamePart(invoice.KSeFNumber),
	)
	return filepath.Join(baseDir, month, kind, fileName)
}

func invoiceHTMLPath(baseDir string, month string, kind string, invoice ksef.InvoiceMetadata) string {
	fileName := fmt.Sprintf("%s_%s_%s_%s.html",
		sanitizeFileNamePart(invoice.IssueDate),
		sanitizeFileNamePart(invoice.Seller.Name),
		sanitizeFileNamePart(invoice.InvoiceNumber),
		sanitizeFileNamePart(invoice.KSeFNumber),
	)
	return filepath.Join(baseDir, month, kind, fileName)
}
