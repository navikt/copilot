package no.nav.benchmark

import java.time.Instant

data class OboToken(val accessToken: String, val expiresAt: Instant) {
    fun isValid(now: Instant): Boolean = now.isBefore(expiresAt)
}
