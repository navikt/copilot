package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

// cmdAuthStatus reports whether the developer is currently logged in,
// re-validating the stored token against GitHub (not just checking presence)
// so a revoked/expired token is reported accurately rather than optimistically.
func cmdAuthStatus(jsonOutput bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, err := currentToken(ctx)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			if jsonOutput {
				return printAuthStatusJSON(authStatus{LoggedIn: false})
			}
			fmt.Printf("  %s Not logged in. Run %s to authenticate.\n", yellow("○"), bold("nav-pilot auth login"))
			return nil
		}
		return fmt.Errorf("reading stored token: %w", err)
	}

	if token.expired() {
		if jsonOutput {
			return printAuthStatusJSON(authStatus{LoggedIn: false, Error: "token expired"})
		}
		fmt.Printf("  %s Token expired on %s. Run %s to re-authenticate.\n", yellow("⚠"), token.ExpiresAt.Format("2006-01-02 15:04"), bold("nav-pilot auth login"))
		return nil
	}

	user, err := fetchGitHubUser(ctx, token.AccessToken)
	if err != nil {
		if jsonOutput {
			return printAuthStatusJSON(authStatus{LoggedIn: false, Error: err.Error()})
		}
		fmt.Printf("  %s Stored token is no longer valid: %v\n", yellow("⚠"), err)
		fmt.Printf("  Run %s to re-authenticate.\n", bold("nav-pilot auth login"))
		return nil
	}

	member, memberErr := checkOrgMembership(ctx, token.AccessToken, navPilotGitHubOrg, user.Login)

	status := authStatus{
		LoggedIn:   true,
		Login:      user.Login,
		Name:       user.Name,
		ObtainedAt: token.ObtainedAt,
		ExpiresAt:  token.ExpiresAt,
	}
	if memberErr != nil {
		// Membership is unknown (not false): leave OrgMember nil and surface
		// the failure via OrgCheckError instead.
		status.OrgCheckError = memberErr.Error()
	} else {
		status.OrgMember = &member
	}

	if jsonOutput {
		return printAuthStatusJSON(status)
	}

	fmt.Printf("  User:     %s", bold(user.Login))
	if user.Name != "" {
		fmt.Printf(" (%s)", user.Name)
	}
	fmt.Println()

	orgLine := fmt.Sprintf("  Org:      %s ", navPilotGitHubOrg)
	switch {
	case memberErr != nil:
		orgLine += fmt.Sprintf("%s (could not verify: %v)", yellow("?"), memberErr)
	case member:
		orgLine += green("✓")
	default:
		orgLine += red("✗ not a member")
	}
	fmt.Println(orgLine)

	fmt.Printf("  Token:    logged in since %s (%s)\n", token.ObtainedAt.Format("2006-01-02 15:04"), formatSecondsRemaining(token.ExpiresAt))
	return nil
}

// githubAppAuthorizations is where a user revokes nav-pilot's access by hand.
const githubAppAuthorizations = "https://github.com/settings/apps/authorizations"

// githubAuthorizations lists both GitHub Apps and OAuth Apps.
const githubAuthorizations = "https://github.com/settings/applications"

// cmdAuthLogout revokes the stored token at GitHub through copilot-cli, then
// removes it from the keychain whatever the revoke's outcome; a failed revoke
// names the page to revoke it by hand (#1274). Idempotent: logging out when
// already logged out is not an error.
func cmdAuthLogout() error {
	revoked := false
	t, err := loadToken()
	if errors.Is(err, keyring.ErrNotFound) {
		fmt.Printf("  %s Not logged in; nothing to remove.\n", yellow("○"))
		return nil
	}
	if err == nil && t.AccessToken != "" && !t.expired() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := revokeToken(ctx, copilotCLIURL(), t.AccessToken)
		cancel()
		if err != nil {
			fmt.Printf("  %s Could not revoke the token at GitHub: %v.\n", yellow("⚠"), err)
			if navPilotGitHubClientID() == navPilotGitHubClientIDDefault {
				fmt.Printf("  To revoke it yourself, remove nav-pilot under Authorized GitHub Apps: %s\n", githubAppAuthorizations)
			} else {
				// An override may name an OAuth App or another App (#1274 review).
				fmt.Printf("  To revoke it yourself, remove the app NAV_PILOT_GITHUB_CLIENT_ID names under Authorized GitHub Apps or Authorized OAuth Apps: %s\n", githubAuthorizations)
			}
		} else {
			revoked = true
		}
	}
	if err := deleteToken(); err != nil {
		return fmt.Errorf("logout failed: %w", err)
	}
	if revoked {
		fmt.Printf("  %s Logged out. Token revoked and removed from OS keychain.\n", green("✓"))
	} else {
		fmt.Printf("  %s Logged out. Token removed from OS keychain.\n", green("✓"))
	}
	return nil
}

// revokeToken asks copilot-cli to revoke token at GitHub, which only the
// App's client secret can do. Only 204 proves the revoke: 401 also covers a
// still-valid token of another App (a NAV_PILOT_GITHUB_CLIENT_ID override).
func revokeToken(ctx context.Context, baseURL, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(baseURL, "/")+"/api/v1/auth/revoke", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling copilot-cli: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return errors.New("copilot-cli does not know the token (already revoked or expired, or issued to another App)")
	}
	return fmt.Errorf("copilot-cli answered %s", resp.Status)
}

type authStatus struct {
	LoggedIn bool   `json:"logged_in"`
	Login    string `json:"login,omitempty"`
	Name     string `json:"name,omitempty"`
	// OrgMember is a pointer so JSON output distinguishes an explicit false
	// (not a member) from nil/omitted (membership check failed — see
	// OrgCheckError).
	OrgMember     *bool     `json:"org_member,omitempty"`
	OrgCheckError string    `json:"org_check_error,omitempty"`
	ObtainedAt    time.Time `json:"obtained_at,omitzero"`
	// ExpiresAt is omitted when the token does not expire (zero value).
	ExpiresAt time.Time `json:"expires_at,omitzero"`
	Error     string    `json:"error,omitempty"`
}

func printAuthStatusJSON(s authStatus) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding auth status: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
