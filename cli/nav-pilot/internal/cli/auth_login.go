package cli

import (
	"context"
	"errors"
	"fmt"
	"time"
)

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
