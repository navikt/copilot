package cli

import (
	"context"
	"errors"
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
func cmdAuthLogin() error { return authLogin(false) }

// authLogin is cmdAuthLogin in English, or in Norwegian (nb) for the survey,
// whose every other line is Norwegian (#1274).
func authLogin(nb bool) error {
	say := func(en, no string) string {
		if nb {
			return no
		}
		return en
	}
	// An override set to the old placeholder does not name an App. Starting the
	// device flow with it just yields a raw GitHub 4xx, so fail fast with an
	// actionable message instead.
	if !hasGitHubApp() {
		return errors.New(say("NAV_PILOT_GITHUB_CLIENT_ID does not name a GitHub App. Unset it to use nav-pilot's own, or set it to the client ID of a GitHub App with device flow enabled",
			"NAV_PILOT_GITHUB_CLIENT_ID peker ikke på noen GitHub App. Fjern variabelen for å bruke nav-pilots egen, eller sett den til klient-ID-en for en GitHub App med device flow slått på"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	fmt.Println()
	token, err := runDeviceFlow(ctx, navPilotGitHubClientID(), navPilotGitHubScopes, func(userCode, verificationURI string) {
		fmt.Printf("  %s "+say("Open %s and enter the code:", "Åpne %s og skriv inn koden:")+"\n\n", bold("→"), bold(verificationURI))
		fmt.Printf("      ┌─────────────┐\n")
		fmt.Printf("      │  %s  │\n", bold(userCode))
		fmt.Printf("      └─────────────┘\n\n")
		fmt.Println(say("  Waiting for approval...", "  Venter på godkjenning …"))
	})
	if err != nil {
		return fmt.Errorf(say("login failed: %w", "innloggingen mislyktes: %w"), err)
	}

	user, err := fetchGitHubUser(ctx, token.AccessToken)
	if err != nil {
		return fmt.Errorf("login succeeded but could not verify identity: %w", err)
	}

	member, err := checkOrgMembership(ctx, token.AccessToken, navPilotGitHubOrg, user.Login)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s "+say("Could not verify %s org membership: %v", "Fikk ikke sjekket om du er medlem av %s: %v")+"\n", yellow("⚠"), navPilotGitHubOrg, err)
	} else if !member {
		fmt.Fprintf(os.Stderr, "%s "+say("You are not a member of the %s GitHub organization — copilot-cli will reject requests until you are.", "Du er ikke medlem av GitHub-organisasjonen %s, og copilot-cli avviser forespørslene dine til du blir det.")+"\n", yellow("⚠"), navPilotGitHubOrg)
	}

	stored := storedToken{Login: user.Login}
	stored.setFrom(token, time.Now())
	if err := saveToken(stored); err != nil {
		return fmt.Errorf("could not store token: %w", err)
	}

	fmt.Println()
	fmt.Printf("  %s "+say("Logged in as %s", "Logget inn som %s")+"\n", green("✓"), bold(user.Login))
	fmt.Println(say("  Token stored securely in your OS keychain.", "  Tokenet er lagret i nøkkelringen på maskinen."))
	fmt.Println()
	return nil
}
