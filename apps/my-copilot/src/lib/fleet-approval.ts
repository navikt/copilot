// Rule text for /fleet (#3071, #3087): who approves, and the message per fleet-control reason code.

/** fleet-control's OperatorTeam (the platform team). The only place my-copilot names it. */
export const PLATFORM_TEAM = "copilot";
export const APPROVER = "team copilot";
export const APPROVAL_RULE =
  "Team copilot godkjenner endringer som gir mer tilgang. Andre endringer blir publisert med en gang.";
export const CHANGE_RULE = "Gir endringen mer tilgang, må team copilot godkjenne den. Ellers gjelder den med en gang.";
export const ownVersionText = (generation: number) =>
  `Versjon ${generation} venter på team copilot. Vil du endre noe før det, bruk «Endre MCP-serveren» nederst på siden. Det blir en ny versjon som erstatter denne.`;
export const waitingText = (generation: number) => `Versjon ${generation} venter på team copilot.`;

export type ReasonContext = { team: string; name: string; generation: number };

/**
 * Message per reason in apps/fleet-control/internal/api/reasons.json; null keeps
 * the message fleet-errors.ts picks by HTTP status. fleet-approval.test.ts keeps
 * the keys and reasons.json in step.
 */
export const REASON_TEXT: Record<string, ((c: ReasonContext) => string) | null> = {
  AlreadyExists: (c) =>
    `Det finnes allerede en MCP-server som heter ${c.name} i ${c.team}. Velg et annet navn, eller åpne den som finnes.`,
  AlreadyGranted: null,
  AlreadyKilling: null,
  ApproverNoOid: () => "Fleet kjenner ikke igjen innloggingen din. Logg ut, logg inn og prøv igjen.",
  BadRequest: null,
  ClientIDInUse: null,
  ConsoleUnavailable: null,
  ContentBlocked: null,
  ContentHashMismatch: null,
  ContentRefused: null,
  Expired: null,
  FromGenerationMismatch: null,
  GenerationBlocked: null,
  GenerationNotPending: null,
  GenerationRevoked: null,
  GrantNotInGeneration: null,
  GrantRevokedSinceApproval: null,
  Invalid: null,
  InvalidSnapshot: null,
  IssuerNotConfigured: null,
  JobNotActive: null,
  KillSwitchOn: null,
  KindMismatch: null,
  LeaseEnded: null,
  MethodNotAllowed: null,
  NameMismatch: null,
  NamespaceMismatch: null,
  NoActiveGrant: null,
  NotFound: null,
  NotOperatorTeam: null,
  NotPlatformTeam: (c) => `Team copilot godkjenner MCP-servere. Versjon ${c.generation} venter på dem.`,
  NotPublished: null,
  NotServerOwnerTeam: null,
  OrdererUnknown: null,
  OverSnapshotBudget: null,
  OverTeamBudget: null,
  PersonTokenRequired: null,
  ReasonRequired: null,
  RegistryNameNotYours: null,
  RegistryNameTaken: null,
  RollbackRevoked: null,
  RollbackTargetNotApproved: null,
  RollbackTargetNotEligible: null,
  RosRequired: null,
  SelfApproval: (c) => `Du sendte inn agentprofilen selv. En annen på ${c.team} må gi tilgang.`,
  ServerRevoked: null,
  ServiceUnavailable: null,
  StaleResourceVersion: null,
  SubmitterNotPerson: (c) =>
    `Ingen person sendte inn versjon ${c.generation}. Noen på ${c.team} må sende den inn på nytt.`,
  UnknownClient: null,
};
