package no.nav.benchmark

import io.ktor.http.HttpStatusCode
import io.ktor.server.application.Application
import io.ktor.server.response.respond
import io.ktor.server.routing.get
import io.ktor.server.routing.routing

data class Note(val id: Int, val text: String)

class NoteRepository(private val notes: List<Note>) {
    init {
        require(notes.zipWithNext().all { (first, second) -> first.id < second.id })
    }

    fun page(after: Int?, limit: Int): List<Note> =
        notes.filter { after == null || it.id >= after }.take(limit)
}

fun Application.notesModule(repository: NoteRepository = NoteRepository(
    listOf(Note(1, "One"), Note(2, "Two"), Note(3, "Three"), Note(4, "Four")),
)) {
    routing {
        get("/notes") {
            val limit = call.request.queryParameters["limit"]?.toIntOrNull() ?: 2
            val after = call.request.queryParameters["after"]?.toIntOrNull()
            if (limit !in 1..100 || (call.request.queryParameters["after"] != null && after == null)) {
                call.respond(HttpStatusCode.BadRequest)
                return@get
            }
            val page = repository.page(after, limit)
            call.respond(page.joinToString("\n") { "${it.id}:${it.text}" })
        }
    }
}
