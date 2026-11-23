package cli

import "fmt"

const budgetURL = "https://ki-utvikling.nav.no/min-side/budsjett"

// cmdBudget only links to the budget page: the copilot-cli gateway does not
// proxy copilot-api's GET budget yet (#2699).
// shortcut: link only, show budget and consumption here once copilot-cli proxies /api/v1/copilot/budget.
func cmdBudget(open bool) error {
	fmt.Println("Se budsjettet ditt for Copilot, hva du har brukt denne måneden og påfyll du har fått, på:")
	fmt.Println("  " + budgetURL)
	fmt.Println("Der ber du også om mer når budsjettet er brukt opp.")
	if open {
		if err := openBrowserFn(budgetURL); err != nil {
			fmt.Println(dim("Fikk ikke åpnet nettleseren. Åpne lenken over selv."))
		}
	}
	return nil
}
