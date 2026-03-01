package live.lcc.android.data.repository

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import live.lcc.android.data.model.Canyon
import live.lcc.android.data.model.MediaItem
import live.lcc.android.data.model.MediaType
import live.lcc.android.data.model.RoadCondition
import live.lcc.android.data.model.UDOTResponse
import live.lcc.android.data.model.WeatherStation
import live.lcc.android.data.remote.ApiResult
import live.lcc.android.data.remote.LccApiClient
import live.lcc.android.domain.YouTubeUrlHelper
import timber.log.Timber

class CanyonRepository(private val apiClient: LccApiClient) {

    private val _lccCanyon = MutableStateFlow<Canyon?>(null)
    val lccCanyon: StateFlow<Canyon?> = _lccCanyon.asStateFlow()

    private val _bccCanyon = MutableStateFlow<Canyon?>(null)
    val bccCanyon: StateFlow<Canyon?> = _bccCanyon.asStateFlow()

    private val _lccMediaItems = MutableStateFlow<List<MediaItem>>(emptyList())
    val lccMediaItems: StateFlow<List<MediaItem>> = _lccMediaItems.asStateFlow()

    private val _bccMediaItems = MutableStateFlow<List<MediaItem>>(emptyList())
    val bccMediaItems: StateFlow<List<MediaItem>> = _bccMediaItems.asStateFlow()

    private val _lccRoadConditions = MutableStateFlow<List<RoadCondition>>(emptyList())
    val lccRoadConditions: StateFlow<List<RoadCondition>> = _lccRoadConditions.asStateFlow()

    private val _bccRoadConditions = MutableStateFlow<List<RoadCondition>>(emptyList())
    val bccRoadConditions: StateFlow<List<RoadCondition>> = _bccRoadConditions.asStateFlow()

    private val _lccWeatherStations = MutableStateFlow<Map<String, WeatherStation>>(emptyMap())
    val lccWeatherStations: StateFlow<Map<String, WeatherStation>> = _lccWeatherStations.asStateFlow()

    private val _bccWeatherStations = MutableStateFlow<Map<String, WeatherStation>>(emptyMap())
    val bccWeatherStations: StateFlow<Map<String, WeatherStation>> = _bccWeatherStations.asStateFlow()

    private val _isLoading = MutableStateFlow(false)
    val isLoading: StateFlow<Boolean> = _isLoading.asStateFlow()

    private val _lccError = MutableStateFlow<String?>(null)
    val lccError: StateFlow<String?> = _lccError.asStateFlow()

    private val _bccError = MutableStateFlow<String?>(null)
    val bccError: StateFlow<String?> = _bccError.asStateFlow()

    suspend fun refreshLcc() {
        val isInitial = _lccMediaItems.value.isEmpty()
        if (isInitial) _isLoading.value = true
        try {
            when (val result = apiClient.fetchCanyon("lcc")) {
                is ApiResult.Success -> {
                    _lccCanyon.value = result.data
                    _lccMediaItems.value = canyonToMediaItems(result.data)
                    _lccError.value = null
                }
                is ApiResult.NotModified -> { /* keep current data */ }
                is ApiResult.Error -> {
                    _lccError.value = result.exception.message
                    Timber.e(result.exception, "Failed to fetch LCC")
                }
            }
        } finally {
            if (isInitial) _isLoading.value = false
        }
    }

    suspend fun refreshBcc() {
        val isInitial = _bccMediaItems.value.isEmpty()
        if (isInitial) _isLoading.value = true
        try {
            when (val result = apiClient.fetchCanyon("bcc")) {
                is ApiResult.Success -> {
                    _bccCanyon.value = result.data
                    _bccMediaItems.value = canyonToMediaItems(result.data)
                    _bccError.value = null
                }
                is ApiResult.NotModified -> { /* keep current data */ }
                is ApiResult.Error -> {
                    _bccError.value = result.exception.message
                    Timber.e(result.exception, "Failed to fetch BCC")
                }
            }
        } finally {
            if (isInitial) _isLoading.value = false
        }
    }

    suspend fun refreshUdot(canyonId: String) {
        when (val result = apiClient.fetchUDOT(canyonId)) {
            is ApiResult.Success -> {
                applyUdotResult(canyonId, result.data)
            }
            is ApiResult.NotModified -> { /* keep current data */ }
            is ApiResult.Error -> {
                Timber.w(result.exception, "Failed to fetch UDOT for $canyonId")
            }
        }
    }

    private fun applyUdotResult(canyonId: String, data: UDOTResponse) {
        when (canyonId.uppercase()) {
            "LCC" -> {
                _lccRoadConditions.value = data.roadConditions
                _lccWeatherStations.value = data.weatherStations
            }
            "BCC" -> {
                _bccRoadConditions.value = data.roadConditions
                _bccWeatherStations.value = data.weatherStations
            }
        }
    }

    private fun canyonToMediaItems(canyon: Canyon): List<MediaItem> {
        return canyon.cameras.map { camera ->
            val url = if (camera.src.startsWith("/")) {
                apiClient.imageUrl(camera.id)
            } else {
                camera.src
            }
            val embedUrl = YouTubeUrlHelper.extractEmbedUrl(url)
            val type = if (embedUrl != null) {
                MediaType.YouTubeVideo(embedUrl)
            } else {
                MediaType.Image
            }
            MediaItem(
                url = url,
                type = type,
                identifier = camera.id,
                alt = camera.alt,
                weatherStationId = camera.weatherStationId,
            )
        }
    }
}
