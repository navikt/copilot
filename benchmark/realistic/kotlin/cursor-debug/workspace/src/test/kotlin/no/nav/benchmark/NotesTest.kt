package no.nav.benchmark

import io.ktor.client.request.get
import io.ktor.client.statement.bodyAsText
import io.ktor.http.HttpStatusCode
import io.ktor.server.testing.testApplication
import kotlin.test.Test
import kotlin.test.assertEquals

class NotesTest {
    @Test
    fun firstPage() = testApplication {
        application { notesModule() }
        assertEquals("1:One\n2:Two", client.get("/notes?limit=2").bodyAsText())
    }

    @Test
    fun invalidLimit() = testApplication {
        application { notesModule() }
        assertEquals(HttpStatusCode.BadRequest, client.get("/notes?limit=0").status)
    }
}
