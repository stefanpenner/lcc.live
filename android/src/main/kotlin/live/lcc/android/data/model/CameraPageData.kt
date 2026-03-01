package live.lcc.android.data.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class CameraPageData(
    @SerialName("Camera") val camera: Camera = Camera(),
    @SerialName("CanyonName") val canyonName: String = "",
    @SerialName("CanyonPath") val canyonPath: String = "",
    @SerialName("ImageURL") val imageURL: String = "",
    @SerialName("WeatherStation") val weatherStation: WeatherStation? = null,
)
