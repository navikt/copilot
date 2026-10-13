package no.nav.vedtak

import com.fasterxml.jackson.annotation.JsonIgnoreProperties
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.fasterxml.jackson.module.kotlin.readValue

enum class Sakstype { ORDINAER, KLAGE, ANKE }

// Hendelsen publiseres på topicen vedtak-fattet og leses av flere team.
@JsonIgnoreProperties(ignoreUnknown = true)
data class VedtakFattet(
    val vedtakId: String,
    val belop: Int,
    val sakstype: Sakstype = Sakstype.ORDINAER,
)

object VedtakSerde {
    private val mapper = jacksonObjectMapper()

    fun les(json: String): VedtakFattet = mapper.readValue(json)

    fun skriv(hendelse: VedtakFattet): String = mapper.writeValueAsString(hendelse)
}
