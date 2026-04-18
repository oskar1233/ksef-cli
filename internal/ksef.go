package ksef

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"text/template"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type ChallengeResponse struct {
	Challenge   string `json:"challenge"`
	Timestamp   string `json:"timestamp"`
	TimestampMs int64  `json:"timestampMs"`
	ClientIP    string `json:"clientIp"`
}

type TokenInfo struct {
	Token      string `json:"token"`
	ValidUntil string `json:"validUntil"`
}

type AuthenticationInitResponse struct {
	ReferenceNumber     string    `json:"referenceNumber"`
	AuthenticationToken TokenInfo `json:"authenticationToken"`
}

type AuthenticationMethodInfo struct {
	Category    string `json:"category"`
	Code        string `json:"code"`
	DisplayName string `json:"displayName"`
}

type StatusInfo struct {
	Code        int      `json:"code"`
	Description string   `json:"description"`
	Details     []string `json:"details,omitempty"`
}

type AuthenticationOperationStatusResponse struct {
	StartDate                string                   `json:"startDate"`
	AuthenticationMethod     string                   `json:"authenticationMethod,omitempty"`
	AuthenticationMethodInfo AuthenticationMethodInfo `json:"authenticationMethodInfo"`
	Status                   StatusInfo               `json:"status"`
	IsTokenRedeemed          *bool                    `json:"isTokenRedeemed,omitempty"`
	LastTokenRefreshDate     string                   `json:"lastTokenRefreshDate,omitempty"`
	RefreshTokenValidUntil   string                   `json:"refreshTokenValidUntil,omitempty"`
}

type AuthenticationTokensResponse struct {
	AccessToken  TokenInfo `json:"accessToken"`
	RefreshToken TokenInfo `json:"refreshToken"`
}

type AuthenticationTokenRefreshResponse struct {
	AccessToken TokenInfo `json:"accessToken"`
}

type AuthenticationContextIdentifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type InitTokenAuthenticationRequest struct {
	Challenge         string                          `json:"challenge"`
	ContextIdentifier AuthenticationContextIdentifier `json:"contextIdentifier"`
	EncryptedToken    string                          `json:"encryptedToken"`
}

type PublicKeyCertificate struct {
	Certificate string   `json:"certificate"`
	ValidFrom   string   `json:"validFrom"`
	ValidTo     string   `json:"validTo"`
	Usage       []string `json:"usage"`
}

type GenerateTokenRequest struct {
	Permissions []string `json:"permissions"`
	Description string   `json:"description"`
}

type GenerateTokenResponse struct {
	ReferenceNumber string `json:"referenceNumber"`
	Token           string `json:"token"`
}

type TokenStatusResponse struct {
	ReferenceNumber      string                          `json:"referenceNumber"`
	AuthorIdentifier     AuthenticationContextIdentifier `json:"authorIdentifier"`
	ContextIdentifier    AuthenticationContextIdentifier `json:"contextIdentifier"`
	Description          string                          `json:"description"`
	RequestedPermissions []string                        `json:"requestedPermissions"`
	DateCreated          string                          `json:"dateCreated"`
	LastUseDate          string                          `json:"lastUseDate,omitempty"`
	Status               string                          `json:"status"`
	StatusDetails        []string                        `json:"statusDetails,omitempty"`
}

type InvoiceQueryDateRange struct {
	DateType                          string `json:"dateType"`
	From                              string `json:"from"`
	To                                string `json:"to,omitempty"`
	RestrictToPermanentStorageHwmDate *bool  `json:"restrictToPermanentStorageHwmDate,omitempty"`
}

type InvoiceQueryFilters struct {
	SubjectType string                `json:"subjectType"`
	DateRange   InvoiceQueryDateRange `json:"dateRange"`
}

type QueryInvoicesMetadataResponse struct {
	HasMore                 bool              `json:"hasMore"`
	IsTruncated             bool              `json:"isTruncated"`
	PermanentStorageHwmDate string            `json:"permanentStorageHwmDate,omitempty"`
	Invoices                []InvoiceMetadata `json:"invoices"`
}

type InvoiceMetadata struct {
	KSeFNumber           string                `json:"ksefNumber"`
	InvoiceNumber        string                `json:"invoiceNumber"`
	IssueDate            string                `json:"issueDate"`
	InvoicingDate        string                `json:"invoicingDate"`
	AcquisitionDate      string                `json:"acquisitionDate"`
	PermanentStorageDate string                `json:"permanentStorageDate"`
	Seller               InvoiceMetadataSeller `json:"seller"`
	Buyer                InvoiceMetadataBuyer  `json:"buyer"`
	NetAmount            float64               `json:"netAmount"`
	GrossAmount          float64               `json:"grossAmount"`
	VATAmount            float64               `json:"vatAmount"`
	Currency             string                `json:"currency"`
	InvoicingMode        string                `json:"invoicingMode"`
	InvoiceType          string                `json:"invoiceType"`
	HasAttachment        bool                  `json:"hasAttachment"`
}

type InvoiceMetadataSeller struct {
	NIP  string `json:"nip"`
	Name string `json:"name"`
}

type InvoiceMetadataBuyer struct {
	Identifier AuthenticationContextIdentifier `json:"identifier"`
	Name       string                          `json:"name"`
}

const authTokenRequestTemplate = `<?xml version="1.0" encoding="utf-8"?>
<AuthTokenRequest xmlns="http://ksef.mf.gov.pl/auth/token/2.0">
  <Challenge>{{.Challenge}}</Challenge>
  <ContextIdentifier>
    <Nip>{{.NIP}}</Nip>
  </ContextIdentifier>
  <SubjectIdentifierType>{{.SubjectIdentifierType}}</SubjectIdentifierType>
</AuthTokenRequest>
`

func BuildAuthTokenRequestXML(challenge string, nip string, subjectIdentifierType string) (string, error) {
	tmpl, err := template.New("auth-token-request").Parse(authTokenRequestTemplate)
	if err != nil {
		return "", fmt.Errorf("parse auth token request template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		Challenge             string
		NIP                   string
		SubjectIdentifierType string
	}{
		Challenge:             challenge,
		NIP:                   nip,
		SubjectIdentifierType: subjectIdentifierType,
	}); err != nil {
		return "", fmt.Errorf("execute auth token request template: %w", err)
	}

	return buf.String(), nil
}

func (c *Client) Challenge() (*ChallengeResponse, error) {
	var response ChallengeResponse
	if err := c.doJSON(http.MethodPost, "/auth/challenge", nil, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) Authorize(signedXML io.Reader, verifyCertificateChain bool) (*AuthenticationInitResponse, error) {
	query := url.Values{}
	query.Set("verifyCertificateChain", fmt.Sprintf("%t", verifyCertificateChain))

	var response AuthenticationInitResponse
	if err := c.doXML(http.MethodPost, "/auth/xades-signature", signedXML, nil, query, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) AuthStatus(referenceNumber string, authenticationToken string) (*AuthenticationOperationStatusResponse, error) {
	var response AuthenticationOperationStatusResponse
	if err := c.doJSON(http.MethodGet, fmt.Sprintf("/auth/%s", url.PathEscape(referenceNumber)), nil, bearerToken(authenticationToken), nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) AuthTokenRedeem(authenticationToken string) (*AuthenticationTokensResponse, error) {
	var response AuthenticationTokensResponse
	if err := c.doJSON(http.MethodPost, "/auth/token/redeem", nil, bearerToken(authenticationToken), nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) RefreshAccessToken(refreshToken string) (*AuthenticationTokenRefreshResponse, error) {
	var response AuthenticationTokenRefreshResponse
	if err := c.doJSON(http.MethodPost, "/auth/token/refresh", nil, bearerToken(refreshToken), nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetPublicKeyCertificates() ([]PublicKeyCertificate, error) {
	var response []PublicKeyCertificate
	if err := c.doJSON(http.MethodGet, "/security/public-key-certificates", nil, nil, nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) AuthenticateWithKSeFToken(request InitTokenAuthenticationRequest) (*AuthenticationInitResponse, error) {
	var response AuthenticationInitResponse
	if err := c.doJSON(http.MethodPost, "/auth/ksef-token", request, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GenerateToken(accessToken string, request GenerateTokenRequest) (*GenerateTokenResponse, error) {
	var response GenerateTokenResponse
	if err := c.doJSON(http.MethodPost, "/tokens", request, bearerToken(accessToken), nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetTokenStatus(accessToken string, referenceNumber string) (*TokenStatusResponse, error) {
	var response TokenStatusResponse
	if err := c.doJSON(http.MethodGet, fmt.Sprintf("/tokens/%s", url.PathEscape(referenceNumber)), nil, bearerToken(accessToken), nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) QueryInvoicesMetadata(accessToken string, filters InvoiceQueryFilters, sortOrder string, pageOffset int, pageSize int) (*QueryInvoicesMetadataResponse, error) {
	query := url.Values{}
	query.Set("sortOrder", sortOrder)
	query.Set("pageOffset", fmt.Sprintf("%d", pageOffset))
	query.Set("pageSize", fmt.Sprintf("%d", pageSize))

	var response QueryInvoicesMetadataResponse
	if err := c.doJSON(http.MethodPost, "/invoices/query/metadata", filters, bearerToken(accessToken), query, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) DownloadInvoice(accessToken string, ksefNumber string) ([]byte, http.Header, error) {
	req, err := http.NewRequest(http.MethodGet, c.url(fmt.Sprintf("/invoices/ksef/%s", url.PathEscape(ksefNumber)), nil), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", *bearerToken(accessToken))
	req.Header.Set("Accept", "application/xml")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("GET /invoices/ksef/%s failed: %w", ksefNumber, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read invoice body: %w", err)
	}

	if resp.StatusCode > 299 {
		return nil, nil, fmt.Errorf("GET /invoices/ksef/%s failed (%d): %s", ksefNumber, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return body, resp.Header.Clone(), nil
}

func EncryptKSeFToken(token string, timestampMs int64, certificates []PublicKeyCertificate) (string, *PublicKeyCertificate, error) {
	certificate, err := SelectKSeFTokenEncryptionCertificate(certificates)
	if err != nil {
		return "", nil, err
	}

	certBytes, err := base64.StdEncoding.DecodeString(certificate.Certificate)
	if err != nil {
		return "", nil, fmt.Errorf("decode public key certificate: %w", err)
	}

	parsedCertificate, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return "", nil, fmt.Errorf("parse public key certificate: %w", err)
	}

	rsaPublicKey, ok := parsedCertificate.PublicKey.(*rsa.PublicKey)
	if !ok {
		return "", nil, fmt.Errorf("public key certificate does not contain an RSA public key")
	}

	payload := []byte(fmt.Sprintf("%s|%d", token, timestampMs))
	encrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPublicKey, payload, nil)
	if err != nil {
		return "", nil, fmt.Errorf("encrypt token with RSA-OAEP SHA-256: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(encrypted)
	return encoded, certificate, nil
}

func SelectKSeFTokenEncryptionCertificate(certificates []PublicKeyCertificate) (*PublicKeyCertificate, error) {
	matching := make([]PublicKeyCertificate, 0)
	for _, certificate := range certificates {
		if hasUsage(certificate.Usage, "KsefTokenEncryption") {
			matching = append(matching, certificate)
		}
	}

	if len(matching) == 0 {
		return nil, fmt.Errorf("no public certificate with usage KsefTokenEncryption found")
	}

	now := time.Now()
	validNow := make([]PublicKeyCertificate, 0)
	for _, certificate := range matching {
		validFrom, errFrom := time.Parse(time.RFC3339, certificate.ValidFrom)
		validTo, errTo := time.Parse(time.RFC3339, certificate.ValidTo)
		if errFrom == nil && errTo == nil && (now.Equal(validFrom) || now.After(validFrom)) && now.Before(validTo) {
			validNow = append(validNow, certificate)
		}
	}

	selected := matching
	if len(validNow) > 0 {
		selected = validNow
	}

	sort.Slice(selected, func(i, j int) bool {
		return selected[i].ValidTo > selected[j].ValidTo
	})

	certificate := selected[0]
	return &certificate, nil
}

func TokenStillValid(token *TokenInfo, now time.Time) bool {
	if token == nil || strings.TrimSpace(token.Token) == "" || strings.TrimSpace(token.ValidUntil) == "" {
		return false
	}

	validUntil, err := time.Parse(time.RFC3339, token.ValidUntil)
	if err != nil {
		return false
	}

	return now.Add(30 * time.Second).Before(validUntil)
}

func (c *Client) doJSON(method string, path string, requestBody any, authorizationHeader *string, query url.Values, responseBody any) error {
	var bodyReader io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, c.url(path, query), bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if authorizationHeader != nil {
		req.Header.Set("Authorization", *authorizationHeader)
	}

	return c.do(req, responseBody)
}

func (c *Client) doXML(method string, path string, requestBody io.Reader, authorizationHeader *string, query url.Values, responseBody any) error {
	req, err := http.NewRequest(method, c.url(path, query), requestBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Accept", "application/json")
	if authorizationHeader != nil {
		req.Header.Set("Authorization", *authorizationHeader)
	}

	return c.do(req, responseBody)
}

func (c *Client) do(req *http.Request, responseBody any) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s failed: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode > 299 {
		return fmt.Errorf("%s %s failed (%d): %s", req.Method, req.URL.Path, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if responseBody == nil || len(body) == 0 {
		return nil
	}

	if err := json.Unmarshal(body, responseBody); err != nil {
		return fmt.Errorf("decode response body: %w; body: %s", err, strings.TrimSpace(string(body)))
	}

	return nil
}

func (c *Client) url(path string, query url.Values) string {
	fullURL := c.BaseURL + path
	if len(query) == 0 {
		return fullURL
	}
	return fullURL + "?" + query.Encode()
}

func bearerToken(token string) *string {
	value := fmt.Sprintf("Bearer %s", token)
	return &value
}

func hasUsage(usages []string, expected string) bool {
	for _, usage := range usages {
		if usage == expected {
			return true
		}
	}
	return false
}
