package no.nav.vedtak

data class VedtakDto(
    val vedtakId: String,
    val belop: Int,
    val fattet: String,
)

fun Vedtak.tilDto() = VedtakDto(vedtakId, belop, fattet.toString())
