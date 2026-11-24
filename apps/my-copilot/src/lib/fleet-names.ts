// No server imports here: client components use these too (fleet-control.ts pulls in next/headers).

/** Nais team and resource names are DNS labels. Anything else, `..` included, never reaches a URL. */
export const isName = (s: string) => /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/.test(s);

/** True when the condition is there with status "True"; missing or unknown is false. */
export const hasCondition = (obj: { status: { conditions?: { type: string; status: string }[] } }, type: string) =>
  obj.status.conditions?.some((c) => c.type === type && c.status === "True") ?? false;

export const refKey = (r: { namespace: string; name: string }) => `${r.namespace}/${r.name}`;
