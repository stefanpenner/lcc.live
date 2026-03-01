package live.lcc.android.data.model

import kotlinx.serialization.Serializable

@Serializable
data class Camera(
    val id: String = "",
    val kind: String = "",
    val src: String = "",
    val alt: String = "",
    val canyon: String = "",
    val weatherStationId: Int? = null,
)
