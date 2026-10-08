package no.nav.vedtak

import java.time.LocalDate

data class Vedtak(
    val vedtakId: String,
    val belop: Int,
    val fattet: LocalDate,
)
