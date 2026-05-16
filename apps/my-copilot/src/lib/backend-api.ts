/**
 * Backend API client with Azure AD OBO token exchange
 */

const COPILOT_API_URL = process.env.COPILOT_API_URL || "http://copilot-api";

const isLocalDev = !process.env.NAIS_CLUSTER_NAME;

function getCopilotApiAudience(): string {
  const cluster = process.env.NAIS_CLUSTER_NAME;
  if (!cluster) {
    throw new Error("NAIS_CLUSTER_NAME not configured — cannot determine backend API audience");
  }
  return `api://${cluster}.copilot.copilot-api`;
}

interface TokenExchangeResponse {
  access_token: string;
  issued_token_type: string;
  token_type: string;
  expires_in: number;
}

/**
 * Exchange user token for backend API OBO token via Texas sidecar
 */
async function exchangeToken(userToken: string): Promise<string> {
  const endpoint = process.env.NAIS_TOKEN_EXCHANGE_ENDPOINT;
  if (!endpoint) {
    throw new Error("NAIS_TOKEN_EXCHANGE_ENDPOINT not configured");
  }

  const response = await fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      identity_provider: "entra_id",
      target: getCopilotApiAudience(),
      user_token: userToken,
    }),
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(`Token exchange failed (${response.status}): ${text}`);
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

  const response = await fetch(`${COPILOT_API_URL}${path}`, {
    ...options,
    headers,
  });

  if (!response.ok) {
    const contentType = response.headers.get("content-type");
    if (contentType?.includes("application/problem+json")) {
      const problem = await response.json();
      throw new Error(`Backend API error: ${problem.detail || problem.title}`);
    }
    throw new Error(`Backend API returned ${response.status}`);
  }

  return response.json() as Promise<T>;
}

// Export main functions
export { exchangeToken, backendRequest };
