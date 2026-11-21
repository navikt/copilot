import { extractGatewayClientId } from "./auth.js";
import { createApp } from "./server.js";

const env = process.env;
const { server } = createApp({
  introspectionEndpoint: env.NAIS_TOKEN_INTROSPECTION_ENDPOINT ?? "",
  exchangeEndpoint: env.NAIS_TOKEN_EXCHANGE_ENDPOINT ?? "",
  gatewayClientId: extractGatewayClientId(env.AZURE_APP_PRE_AUTHORIZED_APPS, env.NAIS_CLUSTER_NAME),
  downstreamUrl: env.KODEVERK_URL ?? "http://kodeverk-api.team-rocket",
  downstreamScope: env.KODEVERK_SCOPE ?? "",
  fetch,
});
server.listen(Number(env.PORT ?? 8080));
