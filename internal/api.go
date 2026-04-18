package ksef

import (
	"io"
	"net/http"
)

type API interface {
	Challenge() (*ChallengeResponse, error)
	Authorize(signedXML io.Reader, verifyCertificateChain bool) (*AuthenticationInitResponse, error)
	AuthStatus(referenceNumber string, authenticationToken string) (*AuthenticationOperationStatusResponse, error)
	AuthTokenRedeem(authenticationToken string) (*AuthenticationTokensResponse, error)
	RefreshAccessToken(refreshToken string) (*AuthenticationTokenRefreshResponse, error)
	GetPublicKeyCertificates() ([]PublicKeyCertificate, error)
	AuthenticateWithKSeFToken(request InitTokenAuthenticationRequest) (*AuthenticationInitResponse, error)
	GenerateToken(accessToken string, request GenerateTokenRequest) (*GenerateTokenResponse, error)
	GetTokenStatus(accessToken string, referenceNumber string) (*TokenStatusResponse, error)
	QueryInvoicesMetadata(accessToken string, filters InvoiceQueryFilters, sortOrder string, pageOffset int, pageSize int) (*QueryInvoicesMetadataResponse, error)
	DownloadInvoice(accessToken string, ksefNumber string) ([]byte, http.Header, error)
}
