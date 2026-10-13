package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.request.forms.submitForm
import io.ktor.client.statement.bodyAsText
import io.ktor.http.isSuccess
import io.ktor.http.parameters
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import java.time.Clock

class TexasException(message: String) : RuntimeException(message)

/** Bytter innkommende brukertoken mot et TokenX-token via Texas-sidecaren. */
class TexasClient(
    private val http: HttpClient,
    private val clock: Clock,
    private val endpoint: String = "http://texas/api/v1/token/exchange",
) {
    suspend fun exchange(userToken: String, audience: String): OboToken {
        val response = http.submitForm(
            url = endpoint,
            formParameters = parameters {
                append("identity_provider", "tokenx")
                append("target", audience)
                append("user_token", userToken)
            },
        )
        if (!response.status.isSuccess()) {
            throw TexasException("Texas svarte ${response.status.value}")
        }
        val body = Json.parseToJsonElement(response.bodyAsText()).jsonObject
        val accessToken = body.getValue("access_token").jsonPrimitive.content
        val expiresIn = body.getValue("expires_in").jsonPrimitive.long
        return OboToken(accessToken, clock.instant().plusSeconds(expiresIn))
    }
}
