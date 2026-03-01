package live.lcc.android.data.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class RoadCondition(
    @SerialName("Id") val id: Int = 0,
    @SerialName("SourceId") val sourceId: String = "",
    @SerialName("RoadCondition") val roadCondition: String = "",
    @SerialName("WeatherCondition") val weatherCondition: String = "",
    @SerialName("Restriction") val restriction: String = "",
    @SerialName("RoadwayName") val roadwayName: String = "",
    @SerialName("EncodedPolyline") val encodedPolyline: String = "",
    @SerialName("LastUpdated") val lastUpdated: Long = 0,
) {
    fun timeAgo(): String {
        val now = System.currentTimeMillis() / 1000
        val diff = now - lastUpdated
        return when {
            diff < 60 -> "just now"
            diff < 3600 -> "${diff / 60}m ago"
            diff < 86400 -> "${diff / 3600}h ago"
            else -> "${diff / 86400}d ago"
        }
    }
}
