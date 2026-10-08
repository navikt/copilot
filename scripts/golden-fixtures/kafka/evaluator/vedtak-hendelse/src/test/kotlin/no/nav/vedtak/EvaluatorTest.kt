package no.nav.vedtak

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// Evaluator-owned: never copied into the agent's workspace.
class EvaluatorTest {
    @Test
    fun `gammel melding uten sakstype leses som ORDINAER`() {
        val v = VedtakSerde.les("""{"vedtakId":"v1","belop":100}""")
        assertEquals("v1", v.vedtakId)
        assertEquals("ORDINAER", v.sakstype.toString())
    }

    @Test
    fun `ny melding har sakstype`() {
        val json = VedtakSerde.skriv(VedtakSerde.les("""{"vedtakId":"v2","belop":5,"sakstype":"KLAGE"}"""))
        assertTrue(json.contains("\"sakstype\":\"KLAGE\""), json)
    }

    @Test
    fun `ukjente felt fra nyere produsenter ignoreres`() {
        val v = VedtakSerde.les("""{"vedtakId":"v3","belop":1,"sakstype":"ORDINAER","enhet":"4402"}""")
        assertEquals("v3", v.vedtakId)
    }
}
