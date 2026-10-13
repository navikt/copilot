package no.nav.vedtak

import org.apache.kafka.clients.consumer.ConsumerRecord
import org.apache.kafka.clients.consumer.MockConsumer
import org.apache.kafka.clients.consumer.OffsetResetStrategy
import org.apache.kafka.common.TopicPartition
import kotlin.test.Test
import kotlin.test.assertEquals

class VedtakKonsumentTest {
    private val tp = TopicPartition("vedtak", 0)

    private fun consumer(vararg verdier: String) = MockConsumer<String, String>(OffsetResetStrategy.EARLIEST).apply {
        assign(listOf(tp))
        updateBeginningOffsets(mapOf(tp to 0L))
        verdier.forEachIndexed { i, v -> addRecord(ConsumerRecord("vedtak", 0, i.toLong(), "sak-1", v)) }
    }

    @Test
    fun `utbetaler hvert vedtak`() {
        val utbetalt = mutableListOf<String>()
        VedtakKonsument(consumer("e1;100", "e2;200")) { id, _ -> utbetalt += id }.poll()
        assertEquals(listOf("e1", "e2"), utbetalt)
    }
}
