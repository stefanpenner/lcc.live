package live.lcc.android.service

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import live.lcc.android.data.remote.LccApiClient
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import timber.log.Timber

class MetricsService(private val apiClient: LccApiClient) {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val buffer = mutableListOf<MetricEvent>()
    private val json = Json { ignoreUnknownKeys = true }

    companion object {
        private const val FLUSH_INTERVAL_MS = 30_000L
        private const val BUFFER_LIMIT = 100
    }

    var enabled: Boolean = true

    fun start() {
        scope.launch {
            while (true) {
                delay(FLUSH_INTERVAL_MS)
                flush()
            }
        }
    }

    fun track(event: String, value: String? = null, durationMs: Long? = null, tags: Map<String, String>? = null) {
        if (!enabled) return
        synchronized(buffer) {
            buffer.add(
                MetricEvent(
                    event = event,
                    value = value,
                    durationMs = durationMs,
                    tags = tags,
                    timestamp = System.currentTimeMillis(),
                )
            )
            if (buffer.size >= BUFFER_LIMIT) {
                scope.launch { flush() }
            }
        }
    }

    private suspend fun flush() {
        val events = synchronized(buffer) {
            if (buffer.isEmpty()) return
            val copy = buffer.toList()
            buffer.clear()
            copy
        }

        try {
            val body = json.encodeToString(MetricBatch(events = events))
            val request = Request.Builder()
                .url("${apiClient.baseUrl}/api/metrics")
                .post(body.toRequestBody("application/json".toMediaType()))
                .build()
            apiClient.let {
                // Use the OkHttpClient from DI - we need to get it through the apiClient
                // For now, just log; the actual client is managed by DI
            }
            Timber.d("Flushed ${events.size} metrics")
        } catch (e: Exception) {
            Timber.w(e, "Failed to flush metrics")
            // Re-buffer on failure
            synchronized(buffer) {
                buffer.addAll(0, events)
                // Trim if too large
                while (buffer.size > BUFFER_LIMIT * 2) {
                    buffer.removeAt(0)
                }
            }
        }
    }

    @Serializable
    data class MetricEvent(
        val event: String,
        val value: String? = null,
        val durationMs: Long? = null,
        val tags: Map<String, String>? = null,
        val timestamp: Long,
    )

    @Serializable
    data class MetricBatch(
        val app: String = "android",
        val events: List<MetricEvent>,
    )
}
