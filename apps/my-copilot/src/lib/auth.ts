import { cache } from "react";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { introspectToken, parseBearerToken } from "./introspect";

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
    const email = process.env.DEV_USER_EMAIL ?? "dev@nav.no";
    const [firstName = "Dev", lastName = "User"] = email
      .split("@")[0]
      .split(".")
      .map((s) => s.charAt(0).toUpperCase() + s.slice(1));
    return {
      firstName,
      lastName,
      email,
      groups: (process.env.DEV_USER_GROUPS ?? "").split(",").filter(Boolean),
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

  // Names come as "Last, First" from Entra ID. For values without the expected
  // ", " separator (e.g. a mononym) fall back to using the whole name so the
  // firstName is never `undefined`.
  let firstName = "";
  let lastName = "";
  if (claims.name) {
    const parts = claims.name.split(", ");
    if (parts.length >= 2) {
      [lastName, firstName] = parts;
    } else {
      firstName = claims.name;
    }
  }
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
