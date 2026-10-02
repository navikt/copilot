package no.nav.benchmark

import io.ktor.http.HttpStatusCode
import io.ktor.server.application.Application
import io.ktor.server.response.respond
import io.ktor.server.routing.get
import io.ktor.server.routing.routing

fun Application.taskModule(service: TaskService = TaskService(TaskRepository())) {
    routing {
        get("/tasks") {
            call.respond(service.list().joinToString("\n") { "${it.id}:${it.title}:${it.status}" })
        }
        get("/tasks/{id}") {
            val id = call.parameters["id"]?.toIntOrNull()
            if (id == null || id <= 0) {
                call.respond(HttpStatusCode.BadRequest)
                return@get
            }
            val task = service.get(id)
            if (task == null) {
                call.respond(HttpStatusCode.NotFound)
            } else {
                call.respond("${task.id}:${task.title}:${task.status}")
            }
        }
    }
}
