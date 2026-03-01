package live.lcc.android.data.model

import kotlinx.serialization.Serializable

@Serializable
data class UDOTResponse(
    val roadConditions: List<RoadCondition> = emptyList(),
    val weatherStations: Map<String, WeatherStation> = emptyMap(),
    val lastUpdated: Long = 0,
)
