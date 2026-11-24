package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

const (
	maxUpstreamBodyBytes  = 8 << 20 // 8 MiB (GET /team returns all teams unpaginated)
	maxConcurrentUpstream = 2
	consumerID            = "mcp-team-catalog"
)

type upstreamStatusError struct {
	status int
}

func (e *upstreamStatusError) Error() string {
	return fmt.Sprintf("team-catalog-backend returned status %d", e.status)
}

type restResponsePage[T any] struct {
	Content []T `json:"content"`
}

// Upstream DTOs decode only the fields needed for projection.
// PII fields (email, onLeave, startDate, endDate, stale, resourceType, teamPercent,
// contactPersonIdent, teamOwnerIdent, ownerGroupNavidentList, contactAddresses,
// changeStamp, links) are intentionally omitted from decoding.

type upstreamResource struct {
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
	FullName   string `json:"fullName"`
}

type upstreamMember struct {
	// NavIdent is used only internally to match the queried member in get_member_memberships;
	// it is never written to any tool output.
	NavIdent string            `json:"navIdent"`
	Roles    []string          `json:"roles"`
	Resource *upstreamResource `json:"resource"`
}

type upstreamTeam struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	SlackChannel  string           `json:"slackChannel"`
	ProductAreaID string           `json:"productAreaId"`
	AvdelingNomID string           `json:"avdelingNomId"`
	AvdelingNavn  string           `json:"avdelingNavn"`
	ClusterIDs    []string         `json:"clusterIds"`
	TeamType      string           `json:"teamType"`
	NaisTeams     []string         `json:"naisTeams"`
	Members       []upstreamMember `json:"members"`
	Tags          []string         `json:"tags"`
	Status        string           `json:"status"`
}

type upstreamProductArea struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	AvdelingNomID string           `json:"avdelingNomId"`
	AvdelingNavn  string           `json:"avdelingNavn"`
	AreaType      string           `json:"areaType"`
	Description   string           `json:"description"`
	SlackChannel  string           `json:"slackChannel"`
	Tags          []string         `json:"tags"`
	Members       []upstreamMember `json:"members"`
	Status        string           `json:"status"`
	IsDefaultArea bool             `json:"isDefaultArea"`
}

type upstreamCluster struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	SlackChannel  string           `json:"slackChannel"`
	Tags          []string         `json:"tags"`
	ProductAreaID string           `json:"productAreaId"`
	AvdelingNomID string           `json:"avdelingNomId"`
	AvdelingNavn  string           `json:"avdelingNavn"`
	Members       []upstreamMember `json:"members"`
	Status        string           `json:"status"`
}

type upstreamMembership struct {
	Teams        []upstreamTeam        `json:"teams"`
	ProductAreas []upstreamProductArea `json:"productAreas"`
	Clusters     []upstreamCluster     `json:"clusters"`
}

type naisTeam struct {
	Slug         string `json:"slug"`
	Purpose      string `json:"purpose,omitempty"`
	SlackChannel string `json:"slackChannel,omitempty"`
}

type teamCatalogClient struct {
	baseURL    string
	httpClient *http.Client
	cache      *responseCache
}

func newTeamCatalogClient(baseURL string, httpClient *http.Client) *teamCatalogClient {
	return &teamCatalogClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
		cache:      newResponseCache(),
	}
}

func (c *teamCatalogClient) getJSON(ctx context.Context, token, path string, query url.Values, cacheable bool, dst any) error {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	if cacheable {
		if body, ok := c.cache.get(target); ok {
			return decodeTeamCatalogJSON(body, dst)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("build team-catalog request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Nav-Call-Id", randomCallID())
	req.Header.Set("Nav-Consumer-Id", consumerID)

	resp, err := c.httpClient.Do(req) //nolint:gosec // baseURL validated at startup, path segments escaped
	if err != nil {
		return fmt.Errorf("team-catalog-backend request failed")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &upstreamStatusError{status: resp.StatusCode}
	}

	body, err := readLimitedBody(resp.Body, maxUpstreamBodyBytes)
	if err != nil {
		return fmt.Errorf("team-catalog-backend response exceeded %d bytes", maxUpstreamBodyBytes)
	}

	if err := decodeTeamCatalogJSON(body, dst); err != nil {
		return err
	}
	if cacheable {
		c.cache.put(target, body)
	}
	return nil
}

func decodeTeamCatalogJSON(body []byte, dst any) error {
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("invalid JSON from team-catalog-backend")
	}
	return nil
}

func (c *teamCatalogClient) listTeams(ctx context.Context, token, productAreaID, clusterID, status string) ([]upstreamTeam, error) {
	q := url.Values{}
	// Upstream TeamController applies productAreaId else-if clusterId, so when both
	// are requested we query by productAreaId and filter clusterId client-side.
	if productAreaID != "" {
		q.Set("productAreaId", productAreaID)
	} else if clusterID != "" {
		q.Set("clusterId", clusterID)
	}
	if status != "" {
		q.Set("status", status)
	}
	var page restResponsePage[upstreamTeam]
	if err := c.getJSON(ctx, token, "/team", q, true, &page); err != nil {
		return nil, err
	}
	if productAreaID != "" && clusterID != "" {
		filtered := make([]upstreamTeam, 0, len(page.Content))
		for _, t := range page.Content {
			if slices.ContainsFunc(t.ClusterIDs, func(id string) bool {
				return strings.EqualFold(id, clusterID)
			}) {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	}
	return page.Content, nil
}

func (c *teamCatalogClient) searchTeams(ctx context.Context, token, query string) ([]upstreamTeam, error) {
	path := "/team/search/" + url.PathEscape(query)
	var page restResponsePage[upstreamTeam]
	if err := c.getJSON(ctx, token, path, nil, true, &page); err != nil {
		return nil, err
	}
	return page.Content, nil
}

func (c *teamCatalogClient) getTeam(ctx context.Context, token, id string) (upstreamTeam, error) {
	path := "/team/" + url.PathEscape(id)
	var team upstreamTeam
	if err := c.getJSON(ctx, token, path, nil, false, &team); err != nil {
		return upstreamTeam{}, err
	}
	return team, nil
}

func (c *teamCatalogClient) getTeamsByNaisTeam(ctx context.Context, token, naisTeamSlug, status string) ([]upstreamTeam, error) {
	path := "/team/naisteam/" + url.PathEscape(naisTeamSlug)
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	var page restResponsePage[upstreamTeam]
	if err := c.getJSON(ctx, token, path, q, true, &page); err != nil {
		return nil, err
	}
	return page.Content, nil
}

func (c *teamCatalogClient) listProductAreas(ctx context.Context, token, status string) ([]upstreamProductArea, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	var page restResponsePage[upstreamProductArea]
	if err := c.getJSON(ctx, token, "/productarea", q, true, &page); err != nil {
		return nil, err
	}
	return page.Content, nil
}

func (c *teamCatalogClient) getProductArea(ctx context.Context, token, id string) (upstreamProductArea, error) {
	path := "/productarea/" + url.PathEscape(id)
	var area upstreamProductArea
	if err := c.getJSON(ctx, token, path, nil, false, &area); err != nil {
		return upstreamProductArea{}, err
	}
	return area, nil
}

func (c *teamCatalogClient) listClusters(ctx context.Context, token, status string) ([]upstreamCluster, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	var page restResponsePage[upstreamCluster]
	if err := c.getJSON(ctx, token, "/cluster", q, true, &page); err != nil {
		return nil, err
	}
	return page.Content, nil
}

func (c *teamCatalogClient) getCluster(ctx context.Context, token, id string) (upstreamCluster, error) {
	path := "/cluster/" + url.PathEscape(id)
	var cluster upstreamCluster
	if err := c.getJSON(ctx, token, path, nil, false, &cluster); err != nil {
		return upstreamCluster{}, err
	}
	return cluster, nil
}

func (c *teamCatalogClient) getMemberMemberships(ctx context.Context, token, navIdent string) (upstreamMembership, error) {
	path := "/member/membership/" + url.PathEscape(navIdent)
	var m upstreamMembership
	if err := c.getJSON(ctx, token, path, nil, false, &m); err != nil {
		return upstreamMembership{}, err
	}
	return m, nil
}

func (c *teamCatalogClient) getNaisTeam(ctx context.Context, token, teamID string) (naisTeam, error) {
	path := "/naisteam/" + url.PathEscape(teamID)
	var nt naisTeam
	if err := c.getJSON(ctx, token, path, nil, true, &nt); err != nil {
		return naisTeam{}, err
	}
	return nt, nil
}

func (c *teamCatalogClient) searchNaisTeams(ctx context.Context, token, query string) ([]naisTeam, error) {
	path := "/naisteam/search/" + url.PathEscape(query)
	var page restResponsePage[naisTeam]
	if err := c.getJSON(ctx, token, path, nil, true, &page); err != nil {
		return nil, err
	}
	return page.Content, nil
}

func randomCallID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
