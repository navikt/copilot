package no.nav.vedtak

import org.apache.kafka.clients.consumer.Consumer
import java.time.Duration

fun interface Utbetaling {
    fun utbetal(eventId: String, belop: Int)
}

// Wrong on purpose: deduplicates, but still commits before the work is done.
class VedtakKonsument(
    private val consumer: Consumer<String, String>,
    private val utbetaling: Utbetaling,
) {
    private val behandlet = mutableSetOf<String>()

    fun poll() {
        val records = consumer.poll(Duration.ofMillis(100))
        consumer.commitSync()
        for (record in records) {
            val (eventId, belop) = record.value().split(";")
            if (behandlet.add(eventId)) utbetaling.utbetal(eventId, belop.toInt())
        }
    }
}
