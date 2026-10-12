package main

import (
	"net/http"
	"os"
	"slices"
)

// adminGroupEnv names the Entra ID object ID of the Nais team copilot's
// group. The group must also be listed under azure.application.claims.groups
// in .nais/app.yaml, or Entra leaves it out of the token's groups claim.
const adminGroupEnv = "COPILOT_ADMIN_GROUP_ID"

// isAdmin reports whether the user is in the admin group. It fails closed:
// no user, no configured group or no match all give false.
func isAdmin(u *User) bool {
	group := os.Getenv(adminGroupEnv)
	return u != nil && group != "" && slices.Contains(u.Groups, group)
}

// requireAdmin guards cost insight endpoints. Everyone else gets 404, so the
// endpoints do not reveal that they exist. Put it inside authMiddleware.
func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := getUserFromContext(r.Context())
		if !isAdmin(user) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
