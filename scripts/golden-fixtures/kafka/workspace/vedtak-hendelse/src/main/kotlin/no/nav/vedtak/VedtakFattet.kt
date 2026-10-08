package no.nav.vedtak

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.fasterxml.jackson.module.kotlin.readValue

// Hendelsen publiseres på topicen vedtak-fattet og leses av flere team.
data class VedtakFattet(
    val vedtakId: String,
    val belop: Int,
)

object VedtakSerde {
    private val mapper = jacksonObjectMapper()

    fun les(json: String): VedtakFattet = mapper.readValue(json)

    fun skriv(hendelse: VedtakFattet): String = mapper.writeValueAsString(hendelse)
}
