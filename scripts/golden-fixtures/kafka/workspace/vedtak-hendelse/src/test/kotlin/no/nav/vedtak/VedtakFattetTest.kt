package no.nav.vedtak

import kotlin.test.Test
import kotlin.test.assertEquals

class VedtakFattetTest {
    @Test
    fun `leser en hendelse`() {
        assertEquals(VedtakFattet("v1", 100), VedtakSerde.les("""{"vedtakId":"v1","belop":100}"""))
    }
}
