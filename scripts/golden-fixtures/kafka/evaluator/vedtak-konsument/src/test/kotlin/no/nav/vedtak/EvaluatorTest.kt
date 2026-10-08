package no.nav.vedtak

import org.apache.kafka.clients.consumer.ConsumerRecord
import org.apache.kafka.clients.consumer.MockConsumer
import org.apache.kafka.clients.consumer.OffsetResetStrategy
import org.apache.kafka.common.TopicPartition
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// Evaluator-owned: never copied into the agent's workspace.
class EvaluatorTest {
    private val tp = TopicPartition("vedtak", 0)

    private fun consumer(vararg verdier: String) = MockConsumer<String, String>(OffsetResetStrategy.EARLIEST).apply {
        assign(listOf(tp))
        updateBeginningOffsets(mapOf(tp to 0L))
        verdier.forEachIndexed { i, v -> addRecord(ConsumerRecord("vedtak", 0, i.toLong(), "sak-1", v)) }
    }

    @Test
    fun `samme eventId utbetales bare en gang`() {
        val utbetalt = mutableListOf<String>()
        val c = consumer("e1;100", "e1;100", "e2;200")
        val k = VedtakKonsument(c, utbetaling = { id, _ -> utbetalt += id })
        k.poll()
        c.addRecord(ConsumerRecord("vedtak", 0, 3L, "sak-1", "e2;200"))
        k.poll()
        assertEquals(listOf("e1", "e2"), utbetalt)
    }

    @Test
    fun `offset commites etter behandling`() {
        val c = consumer("e1;100", "e2;200")
        VedtakKonsument(c, utbetaling = { _, _ -> }).poll()
        assertEquals(2L, c.committed(setOf(tp))[tp]?.offset())
    }

    @Test
    fun `feilet utbetaling commites ikke`() {
        val c = consumer("e1;100", "e2;200")
        runCatching {
            VedtakKonsument(c, utbetaling = { id, _ -> if (id == "e2") error("nede") }).poll()
        }
        val committed = c.committed(setOf(tp))[tp]?.offset() ?: 0L
        assertTrue(committed <= 1L, "committed $committed past the failed record at offset 1")
    }
}
