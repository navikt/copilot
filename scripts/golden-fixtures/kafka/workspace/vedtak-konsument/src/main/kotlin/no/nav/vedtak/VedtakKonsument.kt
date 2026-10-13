package no.nav.vedtak

import org.apache.kafka.clients.consumer.Consumer
import java.time.Duration

fun interface Utbetaling {
    fun utbetal(eventId: String, belop: Int)
}

// Leser vedtak fra topicen og utbetaler. Verdien i hver melding er "eventId;belop".
class VedtakKonsument(
    private val consumer: Consumer<String, String>,
    private val utbetaling: Utbetaling,
) {
    fun poll() {
        val records = consumer.poll(Duration.ofMillis(100))
        consumer.commitSync()
        for (record in records) {
            val (eventId, belop) = record.value().split(";")
            utbetaling.utbetal(eventId, belop.toInt())
        }
    }
}
