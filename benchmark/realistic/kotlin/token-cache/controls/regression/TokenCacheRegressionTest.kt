package no.nav.benchmark

import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.request.bearerAuth
import io.ktor.client.request.get
import io.ktor.client.statement.bodyAsText
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import io.ktor.server.testing.testApplication
import java.time.Clock
import java.time.Instant
import java.time.ZoneId
import java.time.ZoneOffset
import java.util.concurrent.atomic.AtomicInteger
import kotlin.test.Test
import kotlin.test.assertEquals

private class RegressionClock(var now: Instant) : Clock() {
    override fun getZone(): ZoneId = ZoneOffset.UTC
    override fun withZone(zone: ZoneId): Clock = this
    override fun instant(): Instant = now
}

class TokenCacheRegressionTest {
    private val json = headersOf(HttpHeaders.ContentType, "application/json")

    @Test
    fun cacheTreffKallerIkkeTexasPåNytt() = testApplication {
        val clock = RegressionClock(Instant.parse("2026-03-04T10:00:00Z"))
        val exchanges = AtomicInteger()
        val texas = MockEngine {
            val n = exchanges.incrementAndGet()
            respond("""{"access_token":"obo-$n","expires_in":3600,"token_type":"Bearer"}""", headers = json)
        }
        val vedtak = MockEngine { request -> respond(request.headers[HttpHeaders.Authorization]!!, headers = json) }
        application { vedtakModule(texas, vedtak, clock) }

        assertEquals("Bearer obo-1", client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText())
        clock.now = clock.now.plusSeconds(600)
        assertEquals("Bearer obo-1", client.get("/api/vedtak/2") { bearerAuth("bruker-a") }.bodyAsText())
        assertEquals(1, exchanges.get())
    }

    @Test
    fun utløptTokenVekslesPåNytt() = testApplication {
        val clock = RegressionClock(Instant.parse("2026-03-04T10:00:00Z"))
        val exchanges = AtomicInteger()
        val texas = MockEngine {
            val n = exchanges.incrementAndGet()
            respond("""{"access_token":"obo-$n","expires_in":3600,"token_type":"Bearer"}""", headers = json)
        }
        val vedtak = MockEngine { request -> respond(request.headers[HttpHeaders.Authorization]!!, headers = json) }
        application { vedtakModule(texas, vedtak, clock) }

        client.get("/api/vedtak/1") { bearerAuth("bruker-a") }
        clock.now = clock.now.plusSeconds(3600)
        assertEquals("Bearer obo-2", client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText())
    }

    @Test
    fun manglendeTokenGir401UtenTexasKall() = testApplication {
        val engine = MockEngine { error("skal ikke kalles") }
        application { vedtakModule(engine, engine) }
        assertEquals(HttpStatusCode.Unauthorized, client.get("/api/vedtak/1").status)
    }
}
