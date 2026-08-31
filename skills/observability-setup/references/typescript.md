# Next.js and TypeScript observability

## Faro (Frontend Observability)

```typescript
import { initializeFaro } from "@grafana/faro-web-sdk";

const faro = initializeFaro({
  url: process.env.NEXT_PUBLIC_FARO_URL,
  app: {
    name: "my-app",
    version: process.env.NEXT_PUBLIC_APP_VERSION,
    environment: process.env.NEXT_PUBLIC_ENVIRONMENT,
  },
});

// Track errors
try {
  // Code that might fail
} catch (error) {
  faro.api.pushError(error);
}

// Track events
faro.api.pushEvent("user_action", {
  action: "button_click",
  component: "submit_form",
});
```

## API Route Metrics

```typescript
import { Counter, Histogram } from "prom-client";

const requestCounter = new Counter({
  name: "http_requests_total",
  help: "Total HTTP requests",
  labelNames: ["method", "route", "status"],
});

const requestDuration = new Histogram({
  name: "http_request_duration_seconds",
  help: "HTTP request duration",
  labelNames: ["method", "route"],
});

export async function GET() {
  const end = requestDuration.startTimer({ method: "GET", route: "/api/data" });

  try {
    const data = await fetchData();
    requestCounter.inc({ method: "GET", route: "/api/data", status: "200" });
    return NextResponse.json(data);
  } catch (error) {
    requestCounter.inc({ method: "GET", route: "/api/data", status: "500" });
    throw error;
  } finally {
    end();
  }
}
```
