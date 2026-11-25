package types

// TeamMember is one (team, login) pair.
type TeamMember struct {
	TeamSlug string `bigquery:"team_slug"`
	Login    string `bigquery:"login"`
}
