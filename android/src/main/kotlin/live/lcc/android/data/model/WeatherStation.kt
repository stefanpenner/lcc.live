package live.lcc.android.data.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class WeatherStation(
    @SerialName("Id") val id: Int = 0,
    @SerialName("Latitude") val latitude: Double? = null,
    @SerialName("Longitude") val longitude: Double? = null,
    @SerialName("StationName") val stationName: String = "",
    @SerialName("CameraSource") val cameraSource: String? = null,
    @SerialName("CameraSourceId") val cameraSourceId: String? = null,
    @SerialName("AirTemperature") val airTemperature: String? = null,
    @SerialName("SurfaceTemp") val surfaceTemp: String? = null,
    @SerialName("SubSurfaceTemp") val subSurfaceTemp: String? = null,
    @SerialName("SurfaceStatus") val surfaceStatus: String? = null,
    @SerialName("RelativeHumidity") val relativeHumidity: String? = null,
    @SerialName("DewpointTemp") val dewpointTemp: String? = null,
    @SerialName("Precipitation") val precipitation: String? = null,
    @SerialName("WindSpeedAvg") val windSpeedAvg: String? = null,
    @SerialName("WindSpeedGust") val windSpeedGust: String? = null,
    @SerialName("WindDirection") val windDirection: String? = null,
    @SerialName("Source") val source: String = "",
    @SerialName("LastUpdated") val lastUpdated: Long = 0,
) {
    val airTempInt: Int? get() = airTemperature?.toDoubleOrNull()?.toInt()
    val surfaceTempInt: Int? get() = surfaceTemp?.toDoubleOrNull()?.toInt()
    val windSpeedAvgInt: Int? get() = windSpeedAvg?.toDoubleOrNull()?.toInt()
    val windSpeedGustInt: Int? get() = windSpeedGust?.toDoubleOrNull()?.toInt()
}
