package ksef

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
)

const baseUrl = "https://api-test.ksef.mf.gov.pl/v2"

type Client struct {
	authTokenRequestTemplate *template.Template
}

func NewClient() (*Client, error) {
	authTokenRequestTemplate, err := template.New("authTokenRequest").Parse(authTokenRequestTemplateStr)
	if err != nil {
		return nil, fmt.Errorf("parsing auth token request template: %w", err)
	}

	client := Client{authTokenRequestTemplate}
	return &client, nil
}

const authTokenRequestTemplateStr = `
<?xml version="1.0" encoding="UTF-8"?>
<AuthTokenRequest xmlns="http://ksef.mf.gov.pl/schema/gtw/svc/auth/request/2021/10/01/0001">
  <Challenge>{{.Challenge}}</Challenge>
  <ContextIdentifier>
    <Type>nip</Type>
    <Value>{{.Nip}}</Value>
  </ContextIdentifier>
  <SubjectIdentifierType>certificateSubject</SubjectIdentifierType>
</AuthTokenRequest>
`

type ChallengeResp struct {
	Challenge   string `json:"challenge"`
	Timestamp   string `json:"timestamp"`
	TimestampMs int    `json:"timestampMs"`
	ClientIp    string `json:"clientIp"`
}

type AuthTokenRequestTemplateInput struct {
	Challenge string
	Nip       string
}

func (client *Client) Challenge() error {
	resp, err := http.Post(apiUrl("/auth/challenge"), "", bytes.NewBufferString(""))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	challengeResp := ChallengeResp{}
	err = json.NewDecoder(resp.Body).Decode(&challengeResp)
	if err != nil {
		return fmt.Errorf("decoding json: %w", err)
	}

	authTokenRequestTemplateInput := AuthTokenRequestTemplateInput{Challenge: challengeResp.Challenge, Nip: "5170381570"}
	authTokenRequestBodyBuffer := new(bytes.Buffer)
	err = client.authTokenRequestTemplate.Execute(authTokenRequestBodyBuffer, authTokenRequestTemplateInput)
	if err != nil {
		return fmt.Errorf("executing auth token request template: %w", err)
	}

	fmt.Printf("%+v %+v", resp.StatusCode, authTokenRequestBodyBuffer)
	return nil
}

func apiUrl(url string) string {
	return baseUrl + url
}
