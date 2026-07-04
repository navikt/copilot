package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

// cmdAuthStatus reports whether the developer is currently logged in,
// re-validating the stored token against GitHub (not just checking presence)
// so a revoked/expired token is reported accurately rather than optimistically.
func cmdAuthStatus(jsonOutput bool) error {
	token, err := loadToken()
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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
		OrgMember:  member,
		ObtainedAt: token.ObtainedAt,
	}
	if memberErr != nil {
		status.OrgCheckError = memberErr.Error()
	}

	if jsonOutput {
		return printAuthStatusJSON(status)
	}

	fmt.Printf("  Bruker:   %s", bold(user.Login))
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

	fmt.Printf("  Token:    logget inn siden %s (%s)\n", token.ObtainedAt.Format("2006-01-02 15:04"), formatSecondsRemaining(token.ExpiresAt))
	return nil
}

// cmdAuthLogout removes the stored token. Idempotent — logging out when
// already logged out is not an error.
func cmdAuthLogout() error {
	if err := deleteToken(); err != nil {
		return fmt.Errorf("logout failed: %w", err)
	}
	fmt.Printf("  %s Logged out. Token removed from OS keychain.\n", green("✓"))
	return nil
}

type authStatus struct {
	LoggedIn      bool      `json:"logged_in"`
	Login         string    `json:"login,omitempty"`
	Name          string    `json:"name,omitempty"`
	OrgMember     bool      `json:"org_member,omitempty"`
	OrgCheckError string    `json:"org_check_error,omitempty"`
	ObtainedAt    time.Time `json:"obtained_at,omitempty"`
	Error         string    `json:"error,omitempty"`
}

func printAuthStatusJSON(s authStatus) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding auth status: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
