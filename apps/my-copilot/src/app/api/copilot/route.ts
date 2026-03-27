import { getUser } from "@/lib/auth";
import { assignUserToCopilot, getCopilotSeat, getUsernameBySamlIdentity, unassignUserFromCopilot } from "@/lib/github";
import { getLoggerWithTraceContext, getTraceId } from "@/lib/logger";
import { context } from "@opentelemetry/api";
import { NextResponse } from "next/server";
import { revalidateTag } from "next/cache";

export async function GET() {
  const log = getLoggerWithTraceContext(context.active());
  const traceId = getTraceId(context.active());

  const org = "navikt";
  const user = await getUser(false);

  const error = (message: string, status: number) => {
    if (status >= 500) {
      log.error(message);
    } else {
      log.warn(message);
    }

    return NextResponse.json({ error: message, traceId }, { status });
  };

  if (!user) {
    return error("User is not authenticated", 401);
  }

  if (!user.email) {
    return error("User email not found", 500);
  }

  const { user: githubUsername, error: githubError } = await getUsernameBySamlIdentity(user.email, org);

  if (githubError) {
    return error(githubError, 500);
  }

  if (!githubUsername) {
    log.info({ email: user.email }, "GitHub account not linked for user");
    return NextResponse.json({
      githubAccountLinked: false,
      icanhazcopilot: user.groups.length > 0,
    });
  }

  const { copilot: subscription, error: copilotError } = await getCopilotSeat(org, githubUsername);

  if (copilotError) {
    return error(copilotError, 500);
  }

  log.info({ email: user.email }, "User Copilot subscription status");

  return NextResponse.json({
    githubAccountLinked: true,
    icanhazcopilot: user.groups.length > 0,
    subscription,
    githubUsername,
  });
}

enum Action {
  Activate = "activate",
  Deactivate = "deactivate",
}

export async function POST(request: Request) {
  const log = getLoggerWithTraceContext(context.active());
  const traceId = getTraceId(context.active());

  const org = "navikt";
  const user = await getUser(false);

  const error = (message: string, status: number) => {
    if (status >= 500) {
      log.error(message);
    } else {
      log.warn(message);
    }

    return NextResponse.json({ error: message, traceId }, { status });
  };

  if (!user) {
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

  const { user: githubUsername, error: githubError } = await getUsernameBySamlIdentity(user.email, org);

  if (githubError) {
    return error(githubError, 500);
  }

  if (!githubUsername) {
    return error("GitHub username was not found for user email", 400);
  }

  log.info({ email: user.email, action }, "User action on Copilot subscription");

  switch (action) {
    case Action.Activate:
      const { seats_created, error: activateError } = await assignUserToCopilot(org, githubUsername);

      revalidateTag(`status-${githubUsername}`, "max");
      revalidateTag("seats-navikt", "max");
      revalidateTag("billing-navikt", "max");

      if (activateError) {
        return error(activateError, 500);
      }

      return NextResponse.json({ seats_created }, { status: 201 });
    case Action.Deactivate:
      const { seats_cancelled, error: deactivateError } = await unassignUserFromCopilot(org, githubUsername);

      revalidateTag(`status-${githubUsername}`, "max");
      revalidateTag("seats-navikt", "max");
      revalidateTag("billing-navikt", "max");

      if (deactivateError) {
        return error(deactivateError, 500);
      }

      return NextResponse.json({ seats_cancelled }, { status: 200 });
    default:
      return error("Unknown action", 400);
  }
}
