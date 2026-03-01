package live.lcc.android.data.model

import kotlinx.serialization.Serializable

@Serializable
data class Canyon(
    val name: String = "",
    val etag: String = "",
    val status: Camera = Camera(),
    val cameras: List<Camera> = emptyList(),
)
