package no.nav.benchmark

import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.request.bearerAuth
import io.ktor.client.request.get
import io.ktor.client.statement.bodyAsText
import io.ktor.http.HttpHeaders
import io.ktor.http.content.OutgoingContent
import io.ktor.http.headersOf
import io.ktor.http.parseUrlEncodedParameters
import io.ktor.server.testing.testApplication
import java.time.Clock
import java.time.Instant
import java.time.ZoneId
import java.time.ZoneOffset
import java.util.concurrent.atomic.AtomicInteger
import kotlin.test.Test
import kotlin.test.assertEquals

private class AcceptanceClock(var now: Instant) : Clock() {
    override fun getZone(): ZoneId = ZoneOffset.UTC
    override fun withZone(zone: ZoneId): Clock = this
    override fun instant(): Instant = now
}

class TokenCacheAcceptanceTest {
    private val json = headersOf(HttpHeaders.ContentType, "application/json")

    @Test
    fun toBrukereMedSammeAudienceFårHvertSittToken() = testApplication {
        val clock = AcceptanceClock(Instant.parse("2026-03-04T10:00:00Z"))
        val texas = MockEngine { request ->
            val form = String((request.body as OutgoingContent.ByteArrayContent).bytes()).parseUrlEncodedParameters()
            respond("""{"access_token":"obo-for-${form["user_token"]}","expires_in":3600}""", headers = json)
        }
        val vedtak = MockEngine { request -> respond(request.headers[HttpHeaders.Authorization]!!, headers = json) }
        application { vedtakModule(texas, vedtak, clock) }

        assertEquals("Bearer obo-for-bruker-a", client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText())
        assertEquals("Bearer obo-for-bruker-b", client.get("/api/vedtak/1") { bearerAuth("bruker-b") }.bodyAsText())
        assertEquals("Bearer obo-for-bruker-a", client.get("/api/vedtak/2") { bearerAuth("bruker-a") }.bodyAsText())
    }

    @Test
    fun tokenSomSnartUtløperVekslesPåNytt() = testApplication {
        val clock = AcceptanceClock(Instant.parse("2026-03-04T10:00:00Z"))
        val exchanges = AtomicInteger()
        val texas = MockEngine {
            val n = exchanges.incrementAndGet()
            respond("""{"access_token":"obo-$n","expires_in":3600}""", headers = json)
        }
        val vedtak = MockEngine { request -> respond(request.headers[HttpHeaders.Authorization]!!, headers = json) }
        application { vedtakModule(texas, vedtak, clock) }

        assertEquals("Bearer obo-1", client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText())
        clock.now = clock.now.plusSeconds(3600 - 25)
        assertEquals("Bearer obo-2", client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText())
        assertEquals(2, exchanges.get())
    }
}
