package no.nav.benchmark

import io.ktor.client.request.get
import io.ktor.client.request.post
import io.ktor.client.statement.bodyAsText
import io.ktor.http.HttpStatusCode
import io.ktor.server.testing.testApplication
import kotlin.test.Test
import kotlin.test.assertEquals

class CompletionAcceptanceTest {
    @Test
    fun completionPersistsAcrossReadsAndIsIdempotent() = testApplication {
        application { taskModule() }
        val first = client.post("/tasks/1/complete")
        assertEquals(HttpStatusCode.OK, first.status)
        assertEquals("1:Review request:done", first.bodyAsText())
        val repeated = client.post("/tasks/1/complete")
        assertEquals(HttpStatusCode.OK, repeated.status)
        assertEquals("1:Review request:done", repeated.bodyAsText())
        assertEquals("1:Review request:done", client.get("/tasks/1").bodyAsText())
        assertEquals("1:Review request:done\n2:Send response:open", client.get("/tasks").bodyAsText())
    }

    @Test
    fun completionRejectsInvalidAndMissingIds() = testApplication {
        application { taskModule() }
        assertEquals(HttpStatusCode.BadRequest, client.post("/tasks/nope/complete").status)
        assertEquals(HttpStatusCode.BadRequest, client.post("/tasks/0/complete").status)
        assertEquals(HttpStatusCode.NotFound, client.post("/tasks/99/complete").status)
    }
}
