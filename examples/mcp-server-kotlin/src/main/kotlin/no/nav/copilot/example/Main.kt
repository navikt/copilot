package no.nav.copilot.example

import io.ktor.server.engine.embeddedServer
import io.ktor.server.netty.Netty

fun main() {
    val cfg = Config.fromEnv()
    embeddedServer(Netty, port = System.getenv("PORT")?.toIntOrNull() ?: 8080) { module(cfg) }.start(wait = true)
}
