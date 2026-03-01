package live.lcc.android.data.model

import java.util.UUID

sealed class MediaType {
    data object Image : MediaType()
    data class YouTubeVideo(val embedURL: String) : MediaType()
}

data class MediaItem(
    val id: UUID = UUID.randomUUID(),
    val type: MediaType = MediaType.Image,
    val url: String,
    val identifier: String? = null,
    val alt: String? = null,
    val weatherStationId: Int? = null,
) {
    val slug: String?
        get() = alt?.let { slugify(it) }

    val caption: String?
        get() = alt

    val isYouTube: Boolean
        get() = type is MediaType.YouTubeVideo
}

fun slugify(text: String): String {
    return text
        .lowercase()
        .replace(Regex("[\\s_]+"), "-")
        .replace(Regex("[^a-z0-9-]"), "")
        .replace(Regex("-{2,}"), "-")
        .trim('-')
        .ifEmpty { null } ?: ""
}
