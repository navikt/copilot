package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.request.bearerAuth
import io.ktor.client.request.get
import io.ktor.client.statement.HttpResponse

const val ARKIV_AUDIENCE = "dev-gcp:teamdokumenthandtering:arkiv-api"

class ArkivClient(
    private val http: HttpClient,
    private val tokens: TokenCache,
    private val baseUrl: String = "http://arkiv-api",
) {
    suspend fun dokumenter(userToken: String, vedtakId: String): HttpResponse {
        val token = tokens.token(userToken, ARKIV_AUDIENCE)
        return http.get("$baseUrl/api/dokumenter?vedtak=$vedtakId") { bearerAuth(token) }
    }
}
