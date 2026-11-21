package no.nav.copilot.example

import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.LongAdder

/** Per-tool counters in Prometheus text format. Label values come only from registered tool names (bounded cardinality). */
class ToolMetrics(private val tools: Set<String>) {
    private class Stat { val calls = LongAdder(); val errors = LongAdder(); val nanos = LongAdder() }

    private val stats = ConcurrentHashMap<String, Stat>()

    suspend fun <T> record(tool: String, isError: (T) -> Boolean, block: suspend () -> T): T {
        val s = stats.computeIfAbsent(if (tool in tools) tool else "unknown") { Stat() }
        val start = System.nanoTime()
        try {
            val r = block()
            if (isError(r)) s.errors.increment()
            return r
        } catch (e: Exception) {
            s.errors.increment()
            throw e
        } finally {
            s.calls.increment()
            s.nanos.add(System.nanoTime() - start)
        }
    }

    fun render(): String = buildString {
        append("# TYPE mcp_tool_calls_total counter\n# TYPE mcp_tool_errors_total counter\n")
        append("# TYPE mcp_tool_duration_seconds_sum counter\n")
        for ((t, s) in stats.toSortedMap()) {
            append("mcp_tool_calls_total{tool=\"$t\"} ${s.calls.sum()}\n")
            append("mcp_tool_errors_total{tool=\"$t\"} ${s.errors.sum()}\n")
            append("mcp_tool_duration_seconds_sum{tool=\"$t\"} ${s.nanos.sum() / 1e9}\n")
        }
    }
}
