package live.lcc.android.data.repository

import live.lcc.android.data.model.CameraPageData
import live.lcc.android.data.model.WeatherStation
import live.lcc.android.data.remote.ApiResult
import live.lcc.android.data.remote.LccApiClient
import timber.log.Timber

class WeatherRepository(private val apiClient: LccApiClient) {

    // Cache with 30-minute TTL
    private val cache = mutableMapOf<String, CacheEntry>()
    private val fetchedSlugs = mutableSetOf<String>()

    companion object {
        private const val TTL_MS = 30 * 60 * 1000L // 30 minutes
    }

    suspend fun getWeatherStation(slug: String): WeatherStation? {
        // Check cache first
        cache[slug]?.let { entry ->
            if (System.currentTimeMillis() - entry.timestamp < TTL_MS) {
                return entry.station
            }
        }

        // Avoid redundant fetches
        if (slug in fetchedSlugs && cache.containsKey(slug)) {
            return cache[slug]?.station
        }

        return fetchWeather(slug)
    }

    private suspend fun fetchWeather(slug: String): WeatherStation? {
        fetchedSlugs.add(slug)
        return when (val result = apiClient.fetchCamera(slug)) {
            is ApiResult.Success -> {
                val station = result.data.weatherStation
                cache[slug] = CacheEntry(station, System.currentTimeMillis())
                station
            }
            is ApiResult.NotModified -> cache[slug]?.station
            is ApiResult.Error -> {
                Timber.w(result.exception, "Failed to fetch weather for $slug")
                null
            }
        }
    }

    fun clearCache() {
        cache.clear()
        fetchedSlugs.clear()
    }

    private data class CacheEntry(
        val station: WeatherStation?,
        val timestamp: Long,
    )
}
