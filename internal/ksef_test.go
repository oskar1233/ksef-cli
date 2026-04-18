package ksef

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildAuthTokenRequestXML(t *testing.T) {
	xmlContent, err := BuildAuthTokenRequestXML("challenge-1", "1234567890", "certificateSubject")
	require.NoError(t, err)
	require.Contains(t, xmlContent, `<AuthTokenRequest xmlns="http://ksef.mf.gov.pl/auth/token/2.0">`)
	require.Contains(t, xmlContent, `<Challenge>challenge-1</Challenge>`)
	require.Contains(t, xmlContent, `<Nip>1234567890</Nip>`)
	require.Contains(t, xmlContent, `<SubjectIdentifierType>certificateSubject</SubjectIdentifierType>`)
}

func TestTokenStillValid(t *testing.T) {
	now := time.Now()
	require.True(t, TokenStillValid(&TokenInfo{Token: "token", ValidUntil: now.Add(time.Hour).Format(time.RFC3339)}, now))
	require.False(t, TokenStillValid(&TokenInfo{Token: "token", ValidUntil: now.Add(10 * time.Second).Format(time.RFC3339)}, now))
	require.False(t, TokenStillValid(&TokenInfo{Token: "", ValidUntil: now.Add(time.Hour).Format(time.RFC3339)}, now))
}

func TestEncryptKSeFToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	certificateTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "KSeF test cert"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, certificateTemplate, certificateTemplate, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	encoded, selectedCertificate, err := EncryptKSeFToken("ksef-token", 1712862000000, []PublicKeyCertificate{{
		Certificate: base64.StdEncoding.EncodeToString(der),
		ValidFrom:   certificateTemplate.NotBefore.Format(time.RFC3339),
		ValidTo:     certificateTemplate.NotAfter.Format(time.RFC3339),
		Usage:       []string{"KsefTokenEncryption"},
	}})
	require.NoError(t, err)
	require.NotNil(t, selectedCertificate)
	require.NotEmpty(t, encoded)

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)

	plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, ciphertext, nil)
	require.NoError(t, err)
	require.Equal(t, "ksef-token|1712862000000", string(plaintext))
}
