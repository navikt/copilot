package no.nav.copilot.example

import com.sun.net.httpserver.HttpExchange
import com.sun.net.httpserver.HttpServer
import io.ktor.server.engine.EmbeddedServer
import io.ktor.server.engine.embeddedServer
import io.ktor.server.netty.Netty
import kotlinx.coroutines.runBlocking
import java.net.InetSocketAddress
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import kotlin.test.AfterTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

private const val GW = "gw-client-123"
private const val USER_TOKEN = "user-token-secret"

class ServerTest {
    private val claims = mutableMapOf<String, String>()
    private var downstreamAuth: String? = null
    private var downstreamBody = """[{"navn":"Postnummer"}]"""

    private val fake = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0).apply {
        createContext("/introspect") { ex ->
            val token = Regex("\"token\":\"([^\"]+)\"").find(ex.requestBody.readBytes().decodeToString())!!.groupValues[1]
            ex.reply(claims[token] ?: """{"active":false}""")
        }
        createContext("/exchange") { ex -> ex.reply("""{"access_token":"obo-token"}""") }
        createContext("/api/v1/kodeverk") { ex ->
            downstreamAuth = ex.requestHeaders.getFirst("Authorization")
            ex.reply(downstreamBody)
        }
        start()
    }
    private val base = "http://127.0.0.1:${fake.address.port}"
    private val cfg = Config(
        introspectionEndpoint = "$base/introspect", exchangeEndpoint = "$base/exchange", gatewayClientId = GW,
        downstreamUrl = base, downstreamScope = "api://dev-gcp.team.kodeverk/.default",
        allowedHosts = listOf("127.0.0.1"),
    )
    private lateinit var app: EmbeddedServer<*, *>
    private val http = HttpClient.newHttpClient()

    private fun start(c: Config = cfg): String {
        app = embeddedServer(Netty, port = 0) { module(c) }.start()
        return "http://127.0.0.1:${runBlocking { app.engine.resolvedConnectors().first().port }}"
    }

    @AfterTest
    fun stop() {
        if (::app.isInitialized) app.stop(0, 0)
        fake.stop(0)
    }

    private fun HttpExchange.reply(body: String) {
        val b = body.toByteArray()
        sendResponseHeaders(200, b.size.toLong())
        responseBody.use { it.write(b) }
    }

    private fun post(url: String, body: String, token: String? = null): HttpResponse<String> {
        val rb = HttpRequest.newBuilder(URI.create("$url/mcp")).POST(HttpRequest.BodyPublishers.ofString(body))
            .header("Content-Type", "application/json").header("Accept", "application/json, text/event-stream")
        if (token != null) rb.header("Authorization", "Bearer $token")
        return http.send(rb.build(), HttpResponse.BodyHandlers.ofString())
    }

    private val list = """{"jsonrpc":"2.0","id":1,"method":"tools/list"}"""
    private fun call(args: String = "{}") =
        """{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"kodeverk_list","arguments":$args}}"""

    private fun user(azp: String = GW) = """{"active":true,"idtyp":"user","NAVident":"Z123456","azp":"$azp"}"""

    @Test
    fun `auth rejects what the Go reference rejects`() {
        val url = start()
        claims["app"] = """{"active":true,"idtyp":"app","NAVident":"Z123456","azp":"$GW"}"""
        claims["role"] = """{"active":true,"roles":["access_as_application"],"NAVident":"Z123456","azp":"$GW"}"""
        claims["noident"] = """{"active":true,"idtyp":"user","azp":"$GW"}"""
        claims["wrongazp"] = user(azp = "other-client")
        assertEquals(401, post(url, list).statusCode())
        assertEquals(401, post(url, list, "unknown-inactive").statusCode())
        for (t in listOf("app", "role", "noident", "wrongazp")) assertEquals(403, post(url, list, t).statusCode(), t)
    }

    @Test
    fun `fails closed when gateway client id is missing`() {
        claims["ok"] = user()
        assertEquals(503, post(start(cfg.copy(gatewayClientId = "")), list, "ok").statusCode())
    }

    @Test
    fun `request body over the limit is rejected`() {
        assertEquals(413, post(start(), "x".repeat(MAX_REQUEST_BYTES + 1), "ok").statusCode())
    }

    @Test
    fun `tool is read-only, uses OBO token downstream, and is counted`() {
        claims[USER_TOKEN] = user()
        val url = start()
        val tools = post(url, list, USER_TOKEN)
        assertEquals(200, tools.statusCode(), tools.body())
        assertTrue(tools.body().contains("\"readOnlyHint\":true"), tools.body())

        val res = post(url, call("""{"filter":"post"}"""), USER_TOKEN)
        assertTrue(res.body().contains("Postnummer"), res.body())
        assertEquals("Bearer obo-token", downstreamAuth) // never the inbound token
        assertFalse(res.body().contains(USER_TOKEN))

        val metrics = http.send(HttpRequest.newBuilder(URI.create("$url/metrics")).build(), HttpResponse.BodyHandlers.ofString()).body()
        assertTrue(metrics.contains("""mcp_tool_calls_total{tool="kodeverk_list"} 1"""), metrics)
    }

    @Test
    fun `result is capped and oversized upstream is an error`() {
        claims[USER_TOKEN] = user()
        val url = start()
        downstreamBody = "a".repeat(MAX_RESULT_BYTES * 2)
        val capped = post(url, call(), USER_TOKEN).body()
        assertTrue(capped.contains("truncated") && capped.length < MAX_RESULT_BYTES + 1024, capped.length.toString())

        downstreamBody = "a".repeat(MAX_UPSTREAM_BYTES + 1)
        val err = post(url, call(), USER_TOKEN).body()
        assertTrue(err.contains("\"isError\":true"), err)
    }

    @Test
    fun `gateway client id is extracted from pre-authorized apps`() {
        val raw = """[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"gw-1"},{"name":"dev-gcp:x:other","clientId":"o"}]"""
        assertEquals("gw-1", extractGatewayClientId(raw, "dev-gcp"))
        assertEquals("", extractGatewayClientId(raw, "prod-gcp"))
        assertEquals("", extractGatewayClientId(raw, ""))
        assertEquals("", extractGatewayClientId("not json", "dev-gcp"))
        val dup = raw.dropLast(1) + "," + """{"name":"dev-gcp:copilot:fleet-gateway","clientId":"gw-2"}]"""
        assertEquals("", extractGatewayClientId(dup, "dev-gcp"))
    }
}
