package no.nav.benchmark

import io.ktor.client.request.get
import io.ktor.client.statement.bodyAsText
import io.ktor.http.HttpStatusCode
import io.ktor.server.testing.testApplication
import kotlin.test.Test
import kotlin.test.assertEquals

class CursorRegressionTest {
    @Test
    fun firstPageAndExplicitLimit() = testApplication {
        application { notesModule() }
        assertEquals("1:One\n2:Two", client.get("/notes").bodyAsText())
        assertEquals("1:One\n2:Two\n3:Three", client.get("/notes?limit=3").bodyAsText())
    }

    @Test
    fun malformedInputsAreRejected() = testApplication {
        application { notesModule() }
        assertEquals(HttpStatusCode.BadRequest, client.get("/notes?after=oops").status)
        assertEquals(HttpStatusCode.BadRequest, client.get("/notes?limit=101").status)
    }
}
