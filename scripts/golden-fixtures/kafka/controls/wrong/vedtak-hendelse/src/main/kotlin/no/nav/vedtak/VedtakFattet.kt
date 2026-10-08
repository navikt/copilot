package no.nav.vedtak

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.fasterxml.jackson.module.kotlin.readValue

// Wrong on purpose: a required field breaks every message already on the topic.
data class VedtakFattet(
    val vedtakId: String,
    val belop: Int,
    val sakstype: String,
)

object VedtakSerde {
    private val mapper = jacksonObjectMapper()

    fun les(json: String): VedtakFattet = mapper.readValue(json)

    fun skriv(hendelse: VedtakFattet): String = mapper.writeValueAsString(hendelse)
}
