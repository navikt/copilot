// Kept free of next/headers and React imports so proxy.ts can use it too.
interface IntrospectionResponse {
  active: boolean;
  error?: string;
  name?: string;
  preferred_username?: string;
  groups?: string[];
  [key: string]: unknown;
}

export function parseBearerToken(authHeader: string | null): string | null {
  if (!authHeader) {
    return null;
  }

  const parts = authHeader.trim().split(/\s+/);
  if (parts.length !== 2 || parts[0].toLowerCase() !== "bearer" || !parts[1]) {
    return null;
  }

  return parts[1];
}

const INTROSPECTION_TIMEOUT_MS = 5000;

export async function introspectToken(token: string): Promise<IntrospectionResponse | null> {
  const endpoint = process.env.NAIS_TOKEN_INTROSPECTION_ENDPOINT;
  if (!endpoint) {
    throw new Error("NAIS_TOKEN_INTROSPECTION_ENDPOINT is not defined");
  }

  // Every authenticated request passes through here; without a timeout a slow
  // Texas sidecar would hang the auth path. Fail closed (return null) on timeout.
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), INTROSPECTION_TIMEOUT_MS);

  try {
    const response = await fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        identity_provider: "entra_id",
        token,
      }),
      signal: controller.signal,
    });

    if (!response.ok) {
      console.error(`Token introspection returned HTTP ${response.status}`);
      return null;
    }

    const result: IntrospectionResponse = await response.json();

    if (result.active !== true) {
      console.error("Token introspection: inactive token:", result.error);
      return null;
    }

    return result;
  } catch (error) {
    console.error("Token introspection request failed:", error);
    return null;
  } finally {
    clearTimeout(timeoutId);
  }
}
