package api

import "time"

// TypedRef points at another resource, as targetRefs in Gateway API.
type TypedRef struct {
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (r TypedRef) key() string { return r.Namespace + "/" + r.Name }

// AgentIdentityV1 is the v1 wire form of AgentIdentity.
type AgentIdentityV1 struct {
	App           string `json:"app"`
	Cluster       string `json:"cluster"`
	EntraClientID string `json:"entraClientID"`
	StsSubject    string `json:"stsSubject,omitempty"`
}

// AgentGrantV1 is the v1 wire form of AgentGrant.
type AgentGrantV1 struct {
	ServerRef    TypedRef `json:"serverRef"`
	AllowedTools []string `json:"allowedTools"`
	MaxRisk      string   `json:"maxRisk"`
}

// AgentQuotaV1 is the v1 wire form of AgentQuota.
type AgentQuotaV1 struct {
	MaxInFlight       int `json:"maxInFlight"`
	RequestsPerMinute int `json:"requestsPerMinute"`
	DailyCallsPerUser int `json:"dailyCallsPerUser"`
	DailyCalls        int `json:"dailyCalls"`
}

// AgentDPIAV1 is the v1 wire form of AgentDPIA.
type AgentDPIAV1 struct {
	Owner     string `json:"owner"`
	Reference string `json:"reference"`
}

// AgentProfileSpec is the spec of an AgentProfile resource.
type AgentProfileSpec struct {
	Identity AgentIdentityV1 `json:"identity"`
	Mode     string          `json:"mode"`
	Grants   []AgentGrantV1  `json:"grants,omitempty"`
	Quota    AgentQuotaV1    `json:"quota"`
	DPIA     *AgentDPIAV1    `json:"dpia,omitempty"`
}

// GrantStatus is the approval state of one grant.
type GrantStatus struct {
	ServerRef    TypedRef   `json:"serverRef"`
	ContentHash  string     `json:"contentHash,omitempty"`
	Granted      bool       `json:"granted"`
	GrantedBy    *Actor     `json:"grantedBy,omitempty"`
	GrantedAt    *time.Time `json:"grantedAt,omitempty"`
	MissingTools []string   `json:"missingTools,omitempty"`
	Reason       string     `json:"reason,omitempty"`
}

// AgentProfileStatus is the status of an AgentProfile resource.
type AgentProfileStatus struct {
	ObservedGeneration   int64                      `json:"observedGeneration,omitempty"`
	PublishedGeneration  int                        `json:"publishedGeneration,omitempty"`
	PublishedContentHash string                     `json:"publishedContentHash,omitempty"`
	ContentHash          string                     `json:"contentHash,omitempty"` // generations/{n} only
	SubmittedBy          *Actor                     `json:"submittedBy,omitempty"`
	ApprovedAt           *time.Time                 `json:"approvedAt,omitempty"`
	IdentityVerifiedAt   *time.Time                 `json:"identityVerifiedAt,omitempty"`
	ExpiresAt            *time.Time                 `json:"expiresAt,omitempty"`
	Pending              *Pending[AgentProfileSpec] `json:"pending,omitempty"`
	// NeedsResubmit: the pending generation has no person as submitter (a
	// seeded draft), so nobody can approve it until a person submits it again.
	NeedsResubmit bool          `json:"needsResubmit,omitempty"`
	Grants        []GrantStatus `json:"grants,omitempty"`
	Conditions    []Condition   `json:"conditions,omitempty"`
}

// AgentProfileV1 is the AgentProfile resource.
type AgentProfileV1 struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Metadata   ObjectMeta         `json:"metadata"`
	Spec       AgentProfileSpec   `json:"spec"`
	Status     AgentProfileStatus `json:"status"`
}
