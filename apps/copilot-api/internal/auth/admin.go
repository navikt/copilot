package auth

import (
	"net/http"
	"os"
	"slices"
)

// AdminGroupEnv names the Entra ID object ID of the Nais team copilot's
// group. The group must also be listed under azure.application.claims.groups
// in .nais/app.yaml, or Entra leaves it out of the token's groups claim.
const AdminGroupEnv = "COPILOT_ADMIN_GROUP_ID"

// IsAdmin reports whether the user is in the admin group. It fails closed:
// no user, no configured group or no match all give false.
func IsAdmin(u *User) bool {
	group := os.Getenv(AdminGroupEnv)
	return u != nil && group != "" && slices.Contains(u.Groups, group)
}

// RequireAdmin guards cost insight endpoints. Everyone else gets 404, so the
// endpoints do not reveal that they exist. Put it inside authMiddleware.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := UserFromContext(r.Context())
		if !IsAdmin(user) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
