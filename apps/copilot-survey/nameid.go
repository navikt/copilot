package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// nameIDClient asks copilot-api for a GitHub login's Nav e-mail (the nameId
// of its SAML SSO identity in navikt), through POST
// /internal/v1/saml/name-id with an M2M token. The GitHub App key the
// lookup needs stays in copilot-api. One try, no retry, no cache: the e-mail
// is only ever held in memory for the dedup hash.
type nameIDClient struct {
	httpClient *http.Client
	url        string
	texas      *texasClient
}

func newNameIDClient(baseURL string, texas *texasClient) *nameIDClient {
	return &nameIDClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		url:        strings.TrimSuffix(baseURL, "/") + "/internal/v1/saml/name-id",
		texas:      texas,
	}
}

// emailFor is surveyAPI.emailFor: the e-mail from the caller's own token, or
// the lookup for a login asserted by copilot-cli.
func (c *nameIDClient) emailFor(ctx context.Context, who *caller) (string, error) {
	if who.email != "" {
		return who.email, nil
	}
	token, err := c.texas.token(ctx)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string]string{"login": who.login})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return "", errNoNavIdentity
	default:
		return "", fmt.Errorf("name-id lookup: status %d", resp.StatusCode)
	}
	var out struct {
		NameID string `json:"name_id"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<10)).Decode(&out); err != nil {
		return "", fmt.Errorf("name-id lookup: bad response")
	}
	return out.NameID, nil
}
