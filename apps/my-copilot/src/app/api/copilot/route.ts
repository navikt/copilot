import { getUser, getUserToken } from "@/lib/auth";
import { backendRequest } from "@/lib/backend-api";
import { getLoggerWithTraceContext, getTraceId } from "@/lib/logger";
import { context } from "@opentelemetry/api";
import { NextResponse } from "next/server";

export async function GET() {
  const log = getLoggerWithTraceContext(context.active());
  const traceId = getTraceId(context.active());

  const user = await getUser(false);
  const token = await getUserToken();

  const error = (message: string, status: number) => {
    if (status >= 500) {
      log.error(message);
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
      log.info({ email: user.email }, "GitHub account not linked for user");
      return NextResponse.json({
        githubAccountLinked: false,
        icanhazcopilot: user.groups.length > 0,
      });
    }

    const subscription = await backendRequest(`/api/v1/copilot/seats/${githubUsername}`, token);

    log.info({ email: user.email }, "User Copilot subscription status");

    return NextResponse.json({
      githubAccountLinked: true,
      icanhazcopilot: user.groups.length > 0,
      subscription,
      githubUsername,
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    return error(message, 500);
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

  const error = (message: string, status: number) => {
    if (status >= 500) {
      log.error(message);
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

    log.info({ email: user.email, action }, "User action on Copilot subscription");

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
    const message = err instanceof Error ? err.message : String(err);
    return error(message, 500);
  }
}
