package no.nav.copilot.example

import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.server.application.Application
import io.ktor.server.application.createApplicationPlugin
import io.ktor.server.application.install
import io.ktor.server.request.header
import io.ktor.server.request.path
import io.ktor.server.response.header
import io.ktor.server.response.respond
import io.ktor.server.response.respondText
import io.ktor.server.routing.get
import io.ktor.server.routing.routing
import io.ktor.util.AttributeKey
import io.modelcontextprotocol.kotlin.sdk.server.Server
import io.modelcontextprotocol.kotlin.sdk.server.ServerOptions
import io.modelcontextprotocol.kotlin.sdk.server.mcpStatelessStreamableHttp
import io.modelcontextprotocol.kotlin.sdk.types.CallToolResult
import io.modelcontextprotocol.kotlin.sdk.types.Implementation
import io.modelcontextprotocol.kotlin.sdk.types.ServerCapabilities
import io.modelcontextprotocol.kotlin.sdk.types.TextContent
import io.modelcontextprotocol.kotlin.sdk.types.ToolAnnotations
import io.modelcontextprotocol.kotlin.sdk.types.ToolSchema
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put
import kotlinx.serialization.json.putJsonObject
import java.net.URI
import java.net.URLEncoder
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.time.Duration

const val MAX_REQUEST_BYTES = 64 * 1024 // inbound JSON-RPC body
const val MAX_UPSTREAM_BYTES = 1 shl 20 // what we read from the downstream API
const val MAX_RESULT_BYTES = 32 * 1024 // what we hand back to the model
const val TOOL = "kodeverk_list"

private val NavIdentKey = AttributeKey<String>("navIdent")
private val TokenKey = AttributeKey<String>("bearer")

/** Auth and size limits run before the MCP transport sees the request. */
private fun authPlugin(texas: Texas) = createApplicationPlugin("NavAuth") {
    onCall { call ->
        if (call.request.path() != "/mcp") return@onCall
        // Chunked bodies have no Content-Length and are refused, so the limit below cannot be bypassed.
        val len = call.request.header(HttpHeaders.ContentLength)?.toLongOrNull()
        if (call.request.local.method.value == "POST" && len == null) {
            return@onCall call.respond(HttpStatusCode.LengthRequired)
        }
        if (len != null && len > MAX_REQUEST_BYTES) return@onCall call.respond(HttpStatusCode.PayloadTooLarge)

        val token = extractBearerToken(call.request.header(HttpHeaders.Authorization))
        if (token == null) {
            call.response.header(HttpHeaders.WWWAuthenticate, "Bearer")
            return@onCall call.respond(HttpStatusCode.Unauthorized)
        }
        // Texas is a blocking HTTP call; keep it off the event loop.
        when (val r = withContext(Dispatchers.IO) { texas.validate(token) }) {
            is AuthResult.Ok -> {
                call.attributes.put(NavIdentKey, r.navIdent)
                call.attributes.put(TokenKey, token)
            }
            AuthResult.Unauthorized -> {
                call.response.header(HttpHeaders.WWWAuthenticate, "Bearer error=\"invalid_token\"")
                call.respond(HttpStatusCode.Unauthorized)
            }
            AuthResult.Forbidden -> call.respond(HttpStatusCode.Forbidden)
            AuthResult.NotConfigured -> call.respond(HttpStatusCode.ServiceUnavailable)
        }
    }
}

fun Application.module(
    cfg: Config = Config.fromEnv(),
    texas: Texas = Texas(cfg),
    http: HttpClient = defaultHttpClient(),
) {
    val metrics = ToolMetrics(setOf(TOOL))
    install(authPlugin(texas))
    routing {
        get("/health") { call.respondText("ok") }
        get("/ready") { call.respondText("ok") }
        get("/metrics") { call.respondText(metrics.render()) }
    }
    // Stateless: one Server per request, so the caller's token lives in the closure, never in a session.
    mcpStatelessStreamableHttp(allowedHosts = cfg.allowedHosts) {
        val token = call.attributes.getOrNull(TokenKey)
        buildServer(cfg, texas, http, metrics, token)
    }
}

private fun buildServer(cfg: Config, texas: Texas, http: HttpClient, metrics: ToolMetrics, token: String?): Server {
    val server = Server(
        Implementation(name = "mcp-server-kotlin", version = "0.1.0"),
        ServerOptions(capabilities = ServerCapabilities(tools = ServerCapabilities.Tools(listChanged = false))),
    )
    server.addTool(
        name = TOOL,
        description = "List code sets (kodeverk) from the downstream API, optionally filtered by name. " +
            "Example: {\"filter\": \"post\"}. Output is capped at 32 KiB; narrow the filter if truncated.",
        inputSchema = ToolSchema(
            properties = buildJsonObject {
                putJsonObject("filter") {
                    put("type", "string")
                    put("maxLength", 100)
                    put("description", "Optional case-insensitive substring filter on the kodeverk name.")
                }
            },
            required = emptyList(),
        ),
        // Hints only; the server enforces read-only by never calling a write endpoint.
        toolAnnotations = ToolAnnotations(
            title = "List kodeverk",
            readOnlyHint = true,
            destructiveHint = false,
            idempotentHint = true,
            openWorldHint = false,
        ),
    ) { req ->
        metrics.record(req.name, isError = { it.isError == true }) {
            val filter = req.arguments?.get("filter")?.jsonPrimitive?.content.orEmpty()
            if (token == null || filter.length > 100) return@record fail("invalid request: filter must be at most 100 characters")
            try {
                CallToolResult(content = listOf(TextContent(callDownstream(cfg, texas, http, token, filter))))
            } catch (e: DownstreamException) {
                fail(e.message ?: "downstream error")
            }
        }
    }
    return server
}

private fun fail(msg: String) = CallToolResult(content = listOf(TextContent(msg)), isError = true)

private suspend fun callDownstream(cfg: Config, texas: Texas, http: HttpClient, userToken: String, filter: String): String =
    withContext(Dispatchers.IO) {
        val obo = texas.exchange(userToken)
        val url = URI.create(cfg.downstreamUrl.trimEnd('/') + "/api/v1/kodeverk?filter=" + URLEncoder.encode(filter, Charsets.UTF_8))
        val req = HttpRequest.newBuilder(url).timeout(Duration.ofSeconds(10)).header("Authorization", "Bearer $obo").GET().build()
        val body = try {
            val resp = http.send(req, HttpResponse.BodyHandlers.ofInputStream())
            resp.body().use {
                if (resp.statusCode() != 200) throw DownstreamException("downstream returned status ${resp.statusCode()}")
                readLimited(it, MAX_UPSTREAM_BYTES)
            }
        } catch (e: DownstreamException) {
            throw e
        } catch (_: Exception) {
            throw DownstreamException("downstream call failed") // no upstream body or token in the message
        }
        capBytes(body, MAX_RESULT_BYTES)
    }

/** Cuts at the byte limit (on a char boundary) and tells the model the result is partial. */
fun capBytes(s: String, limit: Int): String {
    val b = s.toByteArray()
    if (b.size <= limit) return s
    return String(b, 0, limit, Charsets.UTF_8).trimEnd('�') + "\n[truncated: result exceeded $limit bytes; narrow the filter]"
}
