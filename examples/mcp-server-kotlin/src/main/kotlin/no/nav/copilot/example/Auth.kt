package no.nav.copilot.example

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.security.MessageDigest
import java.time.Duration

const val MAX_BEARER_LEN = 16 * 1024
const val MAX_TEXAS_RESP_BYTES = 1 shl 20

sealed interface AuthResult {
    data class Ok(val navIdent: String) : AuthResult
    data object Unauthorized : AuthResult // 401: missing, malformed, inactive or unverifiable token
    data object Forbidden : AuthResult // 403: valid token, wrong claims
    data object NotConfigured : AuthResult // 503: fail closed
    data object BadGateway : AuthResult // 502: Texas unreachable or answered garbage; not the caller's fault
}

class DownstreamException(message: String) : Exception(message)

/** Config from the Nais environment (Texas sidecar endpoints, pre-authorized apps). */
data class Config(
    val introspectionEndpoint: String,
    val exchangeEndpoint: String,
    val gatewayClientId: String,
    val downstreamUrl: String,
    val downstreamScope: String,
    val allowedHosts: List<String>,
) {
    companion object {
        fun fromEnv(env: Map<String, String> = System.getenv()) = Config(
            introspectionEndpoint = env["NAIS_TOKEN_INTROSPECTION_ENDPOINT"].orEmpty(),
            exchangeEndpoint = env["NAIS_TOKEN_EXCHANGE_ENDPOINT"].orEmpty(),
            gatewayClientId = extractGatewayClientId(
                env["AZURE_APP_PRE_AUTHORIZED_APPS"].orEmpty(),
                env["NAIS_CLUSTER_NAME"].orEmpty(),
            ),
            downstreamUrl = env["DOWNSTREAM_URL"].orEmpty(),
            downstreamScope = env["DOWNSTREAM_SCOPE"].orEmpty(),
            allowedHosts = env["ALLOWED_HOSTS"].orEmpty().split(",").map { it.trim() }.filter { it.isNotEmpty() }
                .ifEmpty { listOf("localhost", "127.0.0.1", "[::1]") },
        )
    }
}

/**
 * fleet-gateway's Entra client ID in this cluster, from AZURE_APP_PRE_AUTHORIZED_APPS.
 * Fails closed (empty string) on unset cluster, malformed JSON, or zero/multiple matches.
 */
fun extractGatewayClientId(raw: String, cluster: String): String {
    if (raw.isBlank() || cluster.isBlank()) return ""
    val apps = try {
        Json.parseToJsonElement(raw).jsonArray
    } catch (_: Exception) {
        return ""
    }
    val ids = apps.mapNotNull {
        val o = it as? JsonObject ?: return@mapNotNull null
        val name = o["name"]?.jsonPrimitive?.content
        val id = o["clientId"]?.jsonPrimitive?.content.orEmpty()
        if (name == "${cluster.trim()}:copilot:fleet-gateway" && id.isNotEmpty()) id else null
    }
    return ids.singleOrNull() ?: ""
}

fun extractBearerToken(header: String?): String? {
    if (header == null || header.length <= 7 || header.length > MAX_BEARER_LEN) return null
    if (!header.startsWith("Bearer ", ignoreCase = true)) return null
    val tok = header.substring(7).trim()
    return tok.takeIf { it.isNotEmpty() && tok.none(Char::isWhitespace) }
}

/** Texas (Nais token sidecar): introspection for inbound tokens, OBO exchange for downstream. */
class Texas(private val cfg: Config, private val http: HttpClient = defaultHttpClient()) {

    fun validate(token: String): AuthResult {
        if (cfg.introspectionEndpoint.isEmpty() || cfg.gatewayClientId.isEmpty()) return AuthResult.NotConfigured
        val body = try {
            post(cfg.introspectionEndpoint, buildJsonObject {
                put("identity_provider", "entra_id")
                put("token", token)
            }.toString())
        } catch (_: Exception) {
            return AuthResult.BadGateway // no token is accepted, but this is not an invalid_token
        }
        val claims = try {
            Json.parseToJsonElement(body).jsonObject
        } catch (_: Exception) {
            return AuthResult.BadGateway
        }
        if (claims["active"]?.jsonPrimitive?.boolean != true) return AuthResult.Unauthorized

        val roles = (claims["roles"] as? JsonArray)?.map { it.jsonPrimitive.content }.orEmpty()
        // App tokens (client credentials) have no user behind them.
        if (claims["idtyp"]?.jsonPrimitive?.content.equals("app", ignoreCase = true) ||
            "access_as_application" in roles
        ) return AuthResult.Forbidden

        val navIdent = claims["NAVident"]?.jsonPrimitive?.content?.trim().orEmpty()
        if (navIdent.isEmpty()) return AuthResult.Forbidden

        // Only fleet-gateway may call us; any other caller, even a valid user token, is rejected.
        val azp = claims["azp"]?.jsonPrimitive?.content.orEmpty()
        if (!MessageDigest.isEqual(azp.toByteArray(), cfg.gatewayClientId.toByteArray())) return AuthResult.Forbidden
        return AuthResult.Ok(navIdent)
    }

    /** OBO: new token for the downstream API. The inbound token is never forwarded (no passthrough). */
    fun exchange(userToken: String): String {
        if (cfg.exchangeEndpoint.isEmpty() || cfg.downstreamScope.isEmpty()) {
            throw DownstreamException("downstream token exchange is not configured")
        }
        val body = try {
            post(cfg.exchangeEndpoint, buildJsonObject {
                put("identity_provider", "entra_id")
                put("target", cfg.downstreamScope)
                put("user_token", userToken)
            }.toString())
        } catch (_: Exception) {
            throw DownstreamException("token exchange failed") // never include Texas' body: may echo the token
        }
        val tok = try {
            Json.parseToJsonElement(body).jsonObject["access_token"]?.jsonPrimitive?.content
        } catch (_: Exception) {
            null
        }
        return tok?.takeIf { it.isNotBlank() } ?: throw DownstreamException("invalid token exchange response")
    }

    private fun post(url: String, json: String): String {
        val req = HttpRequest.newBuilder(URI.create(url)).timeout(Duration.ofSeconds(5))
            .header("Content-Type", "application/json").POST(HttpRequest.BodyPublishers.ofString(json)).build()
        val resp = http.send(req, HttpResponse.BodyHandlers.ofInputStream())
        resp.body().use {
            check(resp.statusCode() == 200) { "texas status ${resp.statusCode()}" }
            return readLimited(it, MAX_TEXAS_RESP_BYTES)
        }
    }
}

fun defaultHttpClient(): HttpClient =
    HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(3)).followRedirects(HttpClient.Redirect.NEVER).build()

fun readLimited(s: java.io.InputStream, limit: Int): String {
    val b = s.readNBytes(limit + 1)
    check(b.size <= limit) { "body exceeded $limit bytes" }
    return String(b, Charsets.UTF_8)
}
