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

export async function getUser(shouldRedirect: boolean = true): Promise<User | null> {
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

  if (!authHeader) {
    if (shouldRedirect) {
      redirect(loginEndpoint);
    }
    return null;
  }

  const token = authHeader.replace("Bearer ", "");
  const claims = await introspectToken(token);

  if (!claims) {
    if (shouldRedirect) {
      redirect(loginEndpoint);
    }
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
}

interface IntrospectionResponse {
  active: boolean;
  error?: string;
  name?: string;
  preferred_username?: string;
  groups?: string[];
  [key: string]: unknown;
}

async function introspectToken(token: string): Promise<IntrospectionResponse | null> {
  const endpoint = process.env.NAIS_TOKEN_INTROSPECTION_ENDPOINT;
  if (!endpoint) {
    throw new Error("NAIS_TOKEN_INTROSPECTION_ENDPOINT is not defined");
  }

  try {
    const response = await fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        identity_provider: "entra_id",
        token,
      }),
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
  }
}
