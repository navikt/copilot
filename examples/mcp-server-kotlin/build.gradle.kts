plugins {
    kotlin("jvm") version "2.4.21"
    application
}

dependencies {
    // Versions follow the official SDK's own catalog (kotlin-sdk 0.16.0: Kotlin 2.4.21, Ktor 3.5.2).
    implementation("io.modelcontextprotocol:kotlin-sdk-server:0.16.0")
    implementation("io.ktor:ktor-server-netty:3.5.2")
    implementation("org.slf4j:slf4j-simple:2.0.20")

    testImplementation(kotlin("test"))
}

kotlin { jvmToolchain(21) }

application { mainClass.set("no.nav.copilot.example.MainKt") }

tasks.test { useJUnitPlatform() }
