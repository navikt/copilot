package no.nav.benchmark

import io.ktor.client.request.get
import io.ktor.client.statement.bodyAsText
import io.ktor.http.HttpStatusCode
import io.ktor.server.testing.testApplication
import kotlin.test.Test
import kotlin.test.assertEquals

class TaskRegressionTest {
    @Test
    fun originalGetEndpointsStillWork() = testApplication {
        application { taskModule() }
        assertEquals("1:Review request:open\n2:Send response:open", client.get("/tasks").bodyAsText())
        assertEquals("2:Send response:open", client.get("/tasks/2").bodyAsText())
        assertEquals(HttpStatusCode.BadRequest, client.get("/tasks/abc").status)
        assertEquals(HttpStatusCode.NotFound, client.get("/tasks/99").status)
    }
}
