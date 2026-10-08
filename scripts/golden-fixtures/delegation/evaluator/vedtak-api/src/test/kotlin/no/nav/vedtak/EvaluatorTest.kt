package no.nav.vedtak

import java.time.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals

class EvaluatorTest {
    @Test
    fun `mapper vedtak til dto`() {
        assertEquals(VedtakDto("v1", 100, "2026-10-08"), Vedtak("v1", 100, LocalDate.of(2026, 10, 8)).tilDto())
    }
}
