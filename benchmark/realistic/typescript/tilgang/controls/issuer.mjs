// Evaluator-owned Azure stand-in: a local JWKS endpoint and RS256 tokens.
// Network is disabled in the evaluator container; this only listens on loopback.
import { createServer } from "node:http";
import { exportJWK, generateKeyPair, SignJWT } from "/deps/node_modules/jose/dist/webapi/index.js";

export async function startIssuer() {
  const trusted = await generateKeyPair("RS256");
  const untrusted = await generateKeyPair("RS256");
  const jwk = { ...(await exportJWK(trusted.publicKey)), kid: "k1", alg: "RS256", use: "sig" };
  const server = createServer((_, response) => {
    response.setHeader("content-type", "application/json");
    response.end(JSON.stringify({ keys: [jwk] }));
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const env = {
    AZURE_OPENID_CONFIG_JWKS_URI: `http://127.0.0.1:${server.address().port}/jwks`,
    AZURE_OPENID_CONFIG_ISSUER: "https://login.example/tenant/v2.0",
    AZURE_APP_CLIENT_ID: "saksoversikt",
  };
  const sign = (key, audience = env.AZURE_APP_CLIENT_ID) =>
    new SignJWT({ NAVident: "Z123456" })
      .setProtectedHeader({ alg: "RS256", kid: "k1" })
      .setIssuer(env.AZURE_OPENID_CONFIG_ISSUER)
      .setAudience(audience)
      .setIssuedAt()
      .setExpirationTime("10m")
      .sign(key);
  return {
    env,
    valid: () => sign(trusted.privateKey),
    forged: () => sign(untrusted.privateKey),
    otherAudience: () => sign(trusted.privateKey, "en-annen-app"),
    close: () => {
      server.closeAllConnections();
      server.close();
    },
  };
}

export const bearer = (token) => ({ headers: { authorization: `Bearer ${token}` } });
