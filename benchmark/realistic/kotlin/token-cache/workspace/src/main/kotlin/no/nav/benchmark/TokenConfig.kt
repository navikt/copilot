package no.nav.benchmark

import java.time.Duration

/** Innstillinger for TokenX-veksling. Se docs/adr/0002-tokenx-via-texas.md. */
data class TokenConfig(
    val texasEndpoint: String = "http://texas/api/v1/token/exchange",
    /** Hvor lang tid før utløp et token regnes som brukt opp. */
    val expiryLeeway: Duration = Duration.ofSeconds(30),
)
