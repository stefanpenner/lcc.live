package live.lcc.android.data.remote

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import live.lcc.android.data.model.CameraPageData
import live.lcc.android.data.model.Canyon
import live.lcc.android.data.model.UDOTResponse
import okhttp3.OkHttpClient
import okhttp3.Request
import timber.log.Timber
import java.io.IOException

class LccApiClient(private val client: OkHttpClient) {

    companion object {
        const val DEFAULT_BASE_URL = "https://lcc.live"
    }

    var baseUrl: String = DEFAULT_BASE_URL

    private val json = Json {
        ignoreUnknownKeys = true
        isLenient = true
    }

    // ETag tracking per endpoint
    private val etags = mutableMapOf<String, String>()

    /**
     * Fetch canyon data. Returns null if 304 Not Modified.
     */
    suspend fun fetchCanyon(canyonId: String): ApiResult<Canyon> = withContext(Dispatchers.IO) {
        val path = "/${canyonId.lowercase()}.json"
        fetch(path) { body -> json.decodeFromString<Canyon>(body) }
    }

    /**
     * Fetch UDOT road conditions and weather for a canyon.
     */
    suspend fun fetchUDOT(canyonId: String): ApiResult<UDOTResponse> = withContext(Dispatchers.IO) {
        val path = "/api/canyon/$canyonId/udot"
        fetch(path) { body -> json.decodeFromString<UDOTResponse>(body) }
    }

    /**
     * Fetch camera detail page data.
     */
    suspend fun fetchCamera(slug: String): ApiResult<CameraPageData> = withContext(Dispatchers.IO) {
        val path = "/camera/$slug.json"
        fetch(path) { body -> json.decodeFromString<CameraPageData>(body) }
    }

    /**
     * Check server version via HEAD request. Returns ETag or null.
     */
    suspend fun checkVersion(): String? = withContext(Dispatchers.IO) {
        try {
            val request = Request.Builder()
                .url("$baseUrl/lcc.json")
                .head()
                .build()
            client.newCall(request).execute().use { response ->
                response.header("ETag")
                    ?: response.header("X-Server-Version")
                    ?: response.header("X-Version")
                    ?: response.header("Last-Modified")
            }
        } catch (e: IOException) {
            Timber.w(e, "Version check failed")
            null
        }
    }

    /**
     * Build the full image URL for a camera ID.
     */
    fun imageUrl(cameraId: String): String = "$baseUrl/image/$cameraId"

    private inline fun <T> fetch(path: String, deserialize: (String) -> T): ApiResult<T> {
        val url = "$baseUrl$path"
        val requestBuilder = Request.Builder()
            .url(url)
            .header("Accept", "application/json")

        // Include ETag for conditional request
        etags[path]?.let { etag ->
            requestBuilder.header("If-None-Match", etag)
        }

        return try {
            val response = client.newCall(requestBuilder.build()).execute()
            response.use {
                // Track ETag
                response.header("ETag")?.let { etag ->
                    etags[path] = etag
                }

                when (response.code) {
                    200 -> {
                        val body = response.body?.string()
                            ?: return@use ApiResult.Error(IOException("Empty response body"))
                        try {
                            ApiResult.Success(deserialize(body))
                        } catch (e: Exception) {
                            Timber.e(e, "Failed to parse response from $path")
                            ApiResult.Error(e)
                        }
                    }
                    304 -> ApiResult.NotModified
                    else -> {
                        Timber.w("HTTP ${response.code} from $path")
                        ApiResult.Error(IOException("HTTP ${response.code}"))
                    }
                }
            }
        } catch (e: IOException) {
            Timber.w(e, "Request failed: $path")
            ApiResult.Error(e)
        }
    }
}

sealed class ApiResult<out T> {
    data class Success<T>(val data: T) : ApiResult<T>()
    data object NotModified : ApiResult<Nothing>()
    data class Error(val exception: Exception) : ApiResult<Nothing>()
}
