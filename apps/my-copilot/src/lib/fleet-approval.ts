// Who approves in /fleet after #3071. The full rule-text module with reason codes is #3087.

export const APPROVER = "team copilot";
export const APPROVAL_RULE =
  "Team copilot godkjenner endringer som gir mer tilgang. Andre endringer blir publisert med en gang.";
export const CHANGE_RULE = "Gir endringen mer tilgang, må team copilot godkjenne den. Ellers gjelder den med en gang.";
export const ownVersionText = (generation: number) =>
  `Versjon ${generation} venter på team copilot. Vil du endre noe før det, bruk «Endre MCP-serveren» nederst på siden. Det blir en ny versjon som erstatter denne.`;
