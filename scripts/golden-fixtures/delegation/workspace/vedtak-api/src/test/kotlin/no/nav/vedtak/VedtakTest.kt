package no.nav.vedtak

import java.time.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals

class VedtakTest {
    @Test
    fun `vedtak har id`() {
        assertEquals("v1", Vedtak("v1", 100, LocalDate.of(2026, 10, 8)).vedtakId)
    }
}
