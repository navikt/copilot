package cli

import (
	"context"
	"fmt"
	"os"
	"time"
)

// navPilotGitHubOrg is the org membership nav-pilot verifies after login,
// mirroring copilot-cli's own org check (both must agree since copilot-cli
// re-verifies server-side on every request — this is purely a fast local
// sanity check so the developer finds out immediately, not on first `usage`
// call).
const navPilotGitHubOrg = "navikt"

// cmdAuthLogin runs the GitHub device flow and stores the resulting token in
// the OS keychain (macOS Keychain / Windows Credential Manager / Linux
// libsecret via go-keyring).
func cmdAuthLogin() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	fmt.Println()
	token, err := runDeviceFlow(ctx, navPilotGitHubClientID(), navPilotGitHubScopes, func(userCode, verificationURI string) {
		fmt.Printf("  %s Open %s and enter the code:\n\n", bold("→"), bold(verificationURI))
		fmt.Printf("      ┌─────────────┐\n")
		fmt.Printf("      │  %s  │\n", bold(userCode))
		fmt.Printf("      └─────────────┘\n\n")
		fmt.Println("  Waiting for approval...")
	})
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	user, err := fetchGitHubUser(ctx, token.AccessToken)
	if err != nil {
		return fmt.Errorf("login succeeded but could not verify identity: %w", err)
	}

	member, err := checkOrgMembership(ctx, token.AccessToken, navPilotGitHubOrg, user.Login)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not verify %s org membership: %v\n", yellow("⚠"), navPilotGitHubOrg, err)
	} else if !member {
		fmt.Fprintf(os.Stderr, "%s You are not a member of the %s GitHub organization — copilot-cli will reject requests until you are.\n", yellow("⚠"), navPilotGitHubOrg)
	}

	now := time.Now()
	stored := storedToken{
		AccessToken: token.AccessToken,
		TokenType:   token.TokenType,
		Scope:       token.Scope,
		Login:       user.Login,
		ObtainedAt:  now,
	}
	if token.ExpiresIn > 0 {
		stored.ExpiresAt = now.Add(time.Duration(token.ExpiresIn) * time.Second)
	}
	if err := saveToken(stored); err != nil {
		return fmt.Errorf("could not store token: %w", err)
	}

	fmt.Println()
	fmt.Printf("  %s Logged in as %s\n", green("✓"), bold(user.Login))
	fmt.Println("  Token stored securely in your OS keychain.")
	fmt.Println()
	return nil
}
