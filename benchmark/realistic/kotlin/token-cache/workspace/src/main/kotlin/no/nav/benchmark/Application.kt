package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.engine.HttpClientEngine
import io.ktor.client.engine.cio.CIO
import io.ktor.client.statement.HttpResponse
import io.ktor.client.statement.bodyAsText
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.server.application.Application
import io.ktor.server.application.log
import io.ktor.server.engine.embeddedServer
import io.ktor.server.netty.Netty
import io.ktor.server.response.respond
import io.ktor.server.response.respondText
import io.ktor.server.routing.RoutingCall
import io.ktor.server.routing.get
import io.ktor.server.routing.routing
import java.time.Clock

fun Application.vedtakModule(
    texasEngine: HttpClientEngine,
    vedtakEngine: HttpClientEngine,
    clock: Clock = Clock.systemUTC(),
    arkivEngine: HttpClientEngine = vedtakEngine,
    config: TokenConfig = TokenConfig(),
) {
    val texas = TexasClient(HttpClient(texasEngine), clock, config.texasEndpoint)
    val tokens = TokenCache(texas, clock)
    val vedtak = VedtakClient(HttpClient(vedtakEngine), tokens)
    val arkiv = ArkivClient(HttpClient(arkivEngine), tokens)

    routing {
        get("/internal/isalive") { call.respondText("ALIVE") }
        get("/api/vedtak/{id}") {
            val userToken = call.userToken() ?: return@get call.respond(HttpStatusCode.Unauthorized)
            val id = call.parameters["id"]!!
            call.forward(vedtak.hent(userToken, id), "vedtak-api")
        }
        get("/api/vedtak/{id}/dokumenter") {
            val userToken = call.userToken() ?: return@get call.respond(HttpStatusCode.Unauthorized)
            call.forward(arkiv.dokumenter(userToken, call.parameters["id"]!!), "arkiv-api")
        }
    }
}

private fun RoutingCall.userToken(): String? =
    request.headers[HttpHeaders.Authorization]?.removePrefix("Bearer ")?.takeIf { it.isNotBlank() }

private suspend fun RoutingCall.forward(response: HttpResponse, upstream: String) {
    application.log.info("{} svarte {} for {}", upstream, response.status.value, request.local.uri)
    respondText(response.bodyAsText(), ContentType.Application.Json, response.status)
}

fun main() {
    embeddedServer(Netty, port = 8080) { vedtakModule(CIO.create(), CIO.create(), arkivEngine = CIO.create()) }
        .start(wait = true)
}
