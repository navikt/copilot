package no.nav.benchmark

data class Task(val id: Int, val title: String, val status: String = "open")

class TaskRepository {
    private val tasks = linkedMapOf(1 to Task(1, "Review request"), 2 to Task(2, "Send response"))

    @Synchronized
    fun find(id: Int): Task? = tasks[id]

    @Synchronized
    fun all(): List<Task> = tasks.values.toList()
}
