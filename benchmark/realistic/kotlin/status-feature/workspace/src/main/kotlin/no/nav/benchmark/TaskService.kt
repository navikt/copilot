package no.nav.benchmark

class TaskService(private val repository: TaskRepository) {
    fun get(id: Int): Task? = repository.find(id)

    fun list(): List<Task> = repository.all()
}
