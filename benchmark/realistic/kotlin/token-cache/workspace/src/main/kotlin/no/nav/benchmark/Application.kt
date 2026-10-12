package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.engine.HttpClientEngine
import io.ktor.client.engine.cio.CIO
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
import io.ktor.server.routing.get
import io.ktor.server.routing.routing
import java.time.Clock

fun Application.vedtakModule(
    texasEngine: HttpClientEngine,
    vedtakEngine: HttpClientEngine,
    clock: Clock = Clock.systemUTC(),
) {
    val texas = TexasClient(HttpClient(texasEngine), clock)
    val vedtak = VedtakClient(HttpClient(vedtakEngine), TokenCache(texas, clock))

    routing {
        get("/internal/isalive") { call.respondText("ALIVE") }
        get("/api/vedtak/{id}") {
            val userToken = call.request.headers[HttpHeaders.Authorization]
                ?.removePrefix("Bearer ")
                ?.takeIf { it.isNotBlank() }
            if (userToken == null) {
                call.respond(HttpStatusCode.Unauthorized)
                return@get
            }
            val id = call.parameters["id"]!!
            val response = vedtak.hent(userToken, id)
            call.application.log.info("vedtak-api svarte {} for vedtak {}", response.status.value, id)
            call.respondText(response.bodyAsText(), ContentType.Application.Json, response.status)
        }
    }
}

fun main() {
    embeddedServer(Netty, port = 8080) { vedtakModule(CIO.create(), CIO.create()) }.start(wait = true)
}
