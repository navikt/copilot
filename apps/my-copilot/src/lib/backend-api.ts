/**
 * Backend API client with Azure AD OBO token exchange
 */

const COPILOT_API_URL = process.env.COPILOT_API_URL || "http://copilot-api";
const TOKEN_EXCHANGE_TIMEOUT_MS = 5000;
const BACKEND_REQUEST_TIMEOUT_MS = 15000;

const isLocalDev = !process.env.NAIS_CLUSTER_NAME;

function getCopilotApiAudience(): string {
  const cluster = process.env.NAIS_CLUSTER_NAME;
  if (!cluster) {
    throw new Error("NAIS_CLUSTER_NAME not configured — cannot determine backend API audience");
  }
  // TokenX audience format: api://{cluster}.{app-name}
  // For copilot-api service with clientName: copilot-api
  return `api://${cluster}.copilot-api`;
}

interface TokenExchangeResponse {
  access_token: string;
  issued_token_type: string;
  token_type: string;
  expires_in: number;
}

async function fetchWithTimeout(
  input: RequestInfo | URL,
  init: RequestInit,
  timeoutMs: number,
  timeoutMessage: string
): Promise<Response> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

  try {
    return await fetch(input, { ...init, signal: controller.signal });
  } catch (err) {
    if (err instanceof Error && err.name === "AbortError") {
      throw new Error(timeoutMessage);
    }
    throw err;
  } finally {
    clearTimeout(timeoutId);
  }
}

/**
 * Exchange user token for backend API OBO token via Texas sidecar
 */
async function exchangeToken(userToken: string): Promise<string> {
  const endpoint = process.env.NAIS_TOKEN_EXCHANGE_ENDPOINT;
  if (!endpoint) {
    throw new Error("NAIS_TOKEN_EXCHANGE_ENDPOINT not configured");
  }

  const response = await fetchWithTimeout(
    endpoint,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        identity_provider: "entra_id",
        target: getCopilotApiAudience(),
        user_token: userToken,
      }),
    },
    TOKEN_EXCHANGE_TIMEOUT_MS,
    "Token exchange timed out"
  );

  if (!response.ok) {
    throw new Error(`Token exchange failed (${response.status})`);
  }

  const result: TokenExchangeResponse = await response.json();
  return result.access_token;
}

/**
 * Call backend API with OBO token (or directly in local dev)
 */
async function backendRequest<T>(path: string, userToken: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    ...((options.headers as Record<string, string>) || {}),
    "Content-Type": "application/json",
  };

  if (!isLocalDev) {
    const oboToken = await exchangeToken(userToken);
    headers.Authorization = `Bearer ${oboToken}`;
  }

  const response = await fetchWithTimeout(
    `${COPILOT_API_URL}${path}`,
    {
      ...options,
      headers,
    },
    BACKEND_REQUEST_TIMEOUT_MS,
    "Backend request timed out"
  );

  if (!response.ok) {
    const contentType = response.headers.get("content-type");
    if (contentType?.includes("application/problem+json")) {
      throw new Error(`Backend API error (${response.status})`);
    }
    throw new Error(`Backend API returned ${response.status}`);
  }

  return response.json() as Promise<T>;
}

// Export main functions
export { exchangeToken, backendRequest };
