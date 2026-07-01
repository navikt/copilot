import { getUser, getUserToken } from "@/lib/auth";
import { backendRequest, BackendApiError } from "@/lib/backend-api";
import { getLoggerWithTraceContext, getTraceId } from "@/lib/logger";
import { context } from "@opentelemetry/api";
import { NextResponse } from "next/server";

export async function GET() {
  const log = getLoggerWithTraceContext(context.active());
  const traceId = getTraceId(context.active());

  const user = await getUser(false);
  const token = await getUserToken();

  const error = (message: string, status: number, err?: unknown) => {
    if (status >= 500) {
      log.error({ err }, message);
    } else {
      log.warn(message);
    }

    return NextResponse.json({ error: message, traceId }, { status });
  };

  if (!user || !token) {
    return error("User is not authenticated", 401);
  }

  if (!user.email) {
    return error("User email not found", 500);
  }

  try {
    const samlResponse = await backendRequest<{ identity: string; username: string | null }>(
      `/api/v1/copilot/saml/${encodeURIComponent(user.email)}`,
      token
    );

    const githubUsername = samlResponse.username;

    if (!githubUsername) {
      log.info("GitHub account not linked for user");
      return NextResponse.json({
        githubAccountLinked: false,
        icanhazcopilot: user.groups.length > 0,
      });
    }

    // A 404 from the seat endpoint is a valid state: the user is linked to a
    // GitHub account but has no Copilot seat/license. Treat it as "no
    // subscription" rather than an error so the UI can show the activate state.
    let subscription: unknown = null;
    try {
      subscription = await backendRequest(`/api/v1/copilot/seats/${githubUsername}`, token);
    } catch (err) {
      if (err instanceof BackendApiError && err.status === 404) {
        log.info("User has no Copilot seat");
      } else {
        throw err;
      }
    }

    log.info("User Copilot subscription status fetched");

    return NextResponse.json({
      githubAccountLinked: true,
      icanhazcopilot: user.groups.length > 0,
      subscription,
      githubUsername,
    });
  } catch (err) {
    return error("Failed to fetch Copilot subscription status", 500, err);
  }
}

enum Action {
  Activate = "activate",
  Deactivate = "deactivate",
}

export async function POST(request: Request) {
  const log = getLoggerWithTraceContext(context.active());
  const traceId = getTraceId(context.active());

  const user = await getUser(false);
  const token = await getUserToken();

  const error = (message: string, status: number, err?: unknown) => {
    if (status >= 500) {
      log.error({ err }, message);
    } else {
      log.warn(message);
    }

    return NextResponse.json({ error: message, traceId }, { status });
  };

  if (!user || !token) {
    return error("User is not authenticated", 401);
  }

  if (!user.email) {
    return error("User email not found", 500);
  }

  if (user.groups.length === 0) {
    return error("User is not a member of any groups", 403);
  }

  const { action } = await request.json();

  if (!action) {
    return error("Action is required", 400);
  }

  try {
    const samlResponse = await backendRequest<{ identity: string; username: string | null }>(
      `/api/v1/copilot/saml/${encodeURIComponent(user.email)}`,
      token
    );

    const githubUsername = samlResponse.username;

    if (!githubUsername) {
      return error("GitHub username was not found for user email", 400);
    }

    log.info({ action }, "User action on Copilot subscription");

    // No BFF-level cache to invalidate after mutations: the BFF no longer caches
    // GitHub/billing data (caching is owned by copilot-api). The next GET request
    // will receive fresh data directly from the backend.
    switch (action) {
      case Action.Activate:
        const activateResponse = await backendRequest<{ seats_created: number }>(`/api/v1/copilot/seats`, token, {
          method: "POST",
          body: JSON.stringify({ username: githubUsername }),
        });

        return NextResponse.json({ seats_created: activateResponse.seats_created }, { status: 201 });

      case Action.Deactivate:
        const deactivateResponse = await backendRequest<{ seats_cancelled: number }>(
          `/api/v1/copilot/seats/${githubUsername}`,
          token,
          { method: "DELETE" }
        );

        return NextResponse.json({ seats_cancelled: deactivateResponse.seats_cancelled }, { status: 200 });

      default:
        return error("Unknown action", 400);
    }
  } catch (err) {
    return error("Failed to process Copilot subscription action", 500, err);
  }
}
