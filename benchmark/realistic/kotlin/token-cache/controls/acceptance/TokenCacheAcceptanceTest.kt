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
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.withTimeout
import java.time.Clock
import java.time.Duration
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

    private fun form(body: Any) =
        String((body as OutgoingContent.ByteArrayContent).bytes()).parseUrlEncodedParameters()

    private val echo = MockEngine { request -> respond(request.headers[HttpHeaders.Authorization]!!, headers = json) }

    @Test
    fun toBrukereMedSammeAudienceFårHvertSittToken() = testApplication {
        val clock = AcceptanceClock(Instant.parse("2026-03-04T10:00:00Z"))
        val texas = MockEngine { request ->
            respond("""{"access_token":"obo-for-${form(request.body)["user_token"]}","expires_in":3600}""", headers = json)
        }
        application { vedtakModule(texas, echo, clock) }

        assertEquals("Bearer obo-for-bruker-a", client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText())
        assertEquals("Bearer obo-for-bruker-b", client.get("/api/vedtak/1") { bearerAuth("bruker-b") }.bodyAsText())
        assertEquals("Bearer obo-for-bruker-a", client.get("/api/vedtak/2") { bearerAuth("bruker-a") }.bodyAsText())
    }

    @Test
    fun sammeBrukerFårEgetTokenPerApi() = testApplication {
        val clock = AcceptanceClock(Instant.parse("2026-03-04T10:00:00Z"))
        val texas = MockEngine { request ->
            val form = form(request.body)
            respond("""{"access_token":"${form["user_token"]}@${form["target"]}","expires_in":3600}""", headers = json)
        }
        application { vedtakModule(texas, echo, clock) }

        assertEquals("Bearer bruker-a@$VEDTAK_AUDIENCE", client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText())
        assertEquals("Bearer bruker-a@$ARKIV_AUDIENCE", client.get("/api/vedtak/1/dokumenter") { bearerAuth("bruker-a") }.bodyAsText())
        assertEquals("Bearer bruker-a@$VEDTAK_AUDIENCE", client.get("/api/vedtak/2") { bearerAuth("bruker-a") }.bodyAsText())
    }

    @Test
    fun tokenSomSnartUtløperVekslesPåNytt() = testApplication {
        val clock = AcceptanceClock(Instant.parse("2026-03-04T10:00:00Z"))
        val exchanges = AtomicInteger()
        val texas = MockEngine {
            respond("""{"access_token":"obo-${exchanges.incrementAndGet()}","expires_in":3600}""", headers = json)
        }
        application { vedtakModule(texas, echo, clock) }

        assertEquals("Bearer obo-1", client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText())
        clock.now = clock.now.plusSeconds(3600 - 25)
        assertEquals("Bearer obo-2", client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText())
    }

    @Test
    fun marginenKommerFraKonfigurasjonen() = testApplication {
        val clock = AcceptanceClock(Instant.parse("2026-03-04T10:00:00Z"))
        val exchanges = AtomicInteger()
        val texas = MockEngine {
            respond("""{"access_token":"obo-${exchanges.incrementAndGet()}","expires_in":3600}""", headers = json)
        }
        application { vedtakModule(texas, echo, clock, config = TokenConfig(expiryLeeway = Duration.ofMinutes(2))) }

        client.get("/api/vedtak/1") { bearerAuth("bruker-a") }
        clock.now = clock.now.plusSeconds(3600 - 300)
        assertEquals("Bearer obo-1", client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText())
        clock.now = clock.now.plusSeconds(200)
        assertEquals("Bearer obo-2", client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText())
    }

    @Test
    fun kortMarginGjenbrukerTokenLenger() = testApplication {
        val clock = AcceptanceClock(Instant.parse("2026-03-04T10:00:00Z"))
        val exchanges = AtomicInteger()
        val texas = MockEngine {
            respond("""{"access_token":"obo-${exchanges.incrementAndGet()}","expires_in":3600}""", headers = json)
        }
        application { vedtakModule(texas, echo, clock, config = TokenConfig(expiryLeeway = Duration.ofSeconds(10))) }

        client.get("/api/vedtak/1") { bearerAuth("bruker-a") }
        clock.now = clock.now.plusSeconds(3600 - 20)
        assertEquals("Bearer obo-1", client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText())
    }

    @Test
    fun samtidigeForespørslerDelerÉnVeksling() = testApplication {
        val clock = AcceptanceClock(Instant.parse("2026-03-04T10:00:00Z"))
        val exchanges = AtomicInteger()
        val started = CompletableDeferred<Unit>()
        val release = CompletableDeferred<Unit>()
        val texas = MockEngine {
            val n = exchanges.incrementAndGet()
            started.complete(Unit)
            release.await()
            respond("""{"access_token":"obo-$n","expires_in":3600}""", headers = json)
        }
        application { vedtakModule(texas, echo, clock) }
        startApplication()

        coroutineScope {
            val first = async { client.get("/api/vedtak/1") { bearerAuth("bruker-a") }.bodyAsText() }
            withTimeout(10_000) { started.await() }
            val second = async { client.get("/api/vedtak/2") { bearerAuth("bruker-a") }.bodyAsText() }
            delay(500)
            release.complete(Unit)
            assertEquals("Bearer obo-1", first.await())
            assertEquals("Bearer obo-1", second.await())
        }
        assertEquals(1, exchanges.get())
    }
}
