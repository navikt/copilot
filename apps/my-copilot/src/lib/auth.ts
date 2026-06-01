import { cache } from "react";
import { headers } from "next/headers";
import { redirect } from "next/navigation";

const loginEndpoint = "/oauth2/login";

export type User = {
  firstName: string;
  lastName: string;
  email: string;
  groups: string[];
};

export async function isAuthenticated(): Promise<boolean> {
  const user = await getUser(false);
  return user !== null;
}

// Memoize per request to avoid multiple Texas introspection calls
const getCachedUser = cache(async (): Promise<User | null> => {
  // In development without Texas configured, return mock user
  if (process.env.NODE_ENV === "development" && !process.env.NAIS_TOKEN_INTROSPECTION_ENDPOINT) {
    return {
      firstName: "Hans Kristian",
      lastName: "Flaatten",
      email: "hans.kristian.flaatten@nav.no",
      groups: ["group1", "group2"],
    };
  }

  const authHeader = (await headers()).get("Authorization");
  const token = parseBearerToken(authHeader);
  if (!token) {
    return null;
  }
  const claims = await introspectToken(token);

  if (!claims) {
    return null;
  }

  const [lastName, firstName] = claims.name ? claims.name.split(", ") : ["", ""];
  const email = (claims.preferred_username ?? "").toLowerCase();
  const groups = claims.groups ?? [];

  return {
    firstName,
    lastName,
    email,
    groups,
  };
});

export async function getUser(shouldRedirect: boolean = true): Promise<User | null> {
  const user = await getCachedUser();

  if (!user && shouldRedirect) {
    redirect(loginEndpoint);
  }

  return user;
}

/**
 * Get the raw user token from the Authorization header.
 * This is needed for backend API calls that require token exchange.
 */
export async function getUserToken(): Promise<string | null> {
  // In development without Texas configured, return mock token
  if (process.env.NODE_ENV === "development" && !process.env.NAIS_TOKEN_INTROSPECTION_ENDPOINT) {
    return "mock-dev-token";
  }

  const authHeader = (await headers()).get("Authorization");
  return parseBearerToken(authHeader);
}

interface IntrospectionResponse {
  active: boolean;
  error?: string;
  name?: string;
  preferred_username?: string;
  groups?: string[];
  [key: string]: unknown;
}

function parseBearerToken(authHeader: string | null): string | null {
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

async function introspectToken(token: string): Promise<IntrospectionResponse | null> {
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

    if (!result.active) {
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
