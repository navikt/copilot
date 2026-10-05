package no.nav.benchmark

import io.ktor.client.request.get
import io.ktor.client.statement.bodyAsText
import io.ktor.server.testing.testApplication
import kotlin.test.Test
import kotlin.test.assertEquals

class CursorAcceptanceTest {
    @Test
    fun successivePagesDoNotOverlap() = testApplication {
        application { notesModule() }
        assertEquals("3:Three\n4:Four", client.get("/notes?after=2&limit=2").bodyAsText())
        assertEquals("", client.get("/notes?after=4").bodyAsText())
    }

    @Test
    fun missingCursorValueStillStartsAtBeginning() = testApplication {
        application { notesModule() }
        assertEquals("1:One\n2:Two", client.get("/notes?after=0").bodyAsText())
    }
}
