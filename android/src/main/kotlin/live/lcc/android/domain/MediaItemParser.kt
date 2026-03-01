package live.lcc.android.domain

import live.lcc.android.data.model.MediaItem
import live.lcc.android.data.model.MediaType
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.int
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

object MediaItemParser {

    private val json = Json { ignoreUnknownKeys = true }

    fun parse(jsonString: String): List<MediaItem> {
        val element = json.parseToJsonElement(jsonString)
        return parseElement(element)
    }

    private fun parseElement(element: JsonElement): List<MediaItem> {
        return when (element) {
            is JsonArray -> parseArray(element)
            is JsonObject -> parseObject(element)
            else -> emptyList()
        }
    }

    private fun parseArray(array: JsonArray): List<MediaItem> {
        return array.mapNotNull { item ->
            when (item) {
                is JsonPrimitive -> parseStringItem(item.content)
                is JsonObject -> parseCameraObject(item)
                else -> null
            }
        }
    }

    private fun parseObject(obj: JsonObject): List<MediaItem> {
        // Format 3: { "cameras": [...] }
        obj["cameras"]?.let { cameras ->
            if (cameras is JsonArray) return parseArray(cameras)
        }
        // Format 4: { "images": [...] }
        obj["images"]?.let { images ->
            if (images is JsonArray) return parseArray(images)
        }
        // Single camera object
        return listOfNotNull(parseCameraObject(obj))
    }

    private fun parseStringItem(url: String): MediaItem? {
        if (url.isBlank()) return null
        return MediaItem(
            url = url,
            type = detectMediaType(url),
        )
    }

    private fun parseCameraObject(obj: JsonObject): MediaItem? {
        // Extract URL: try iframe, url, src in order
        val url = obj.stringOrNull("iframe")
            ?: obj.stringOrNull("url")
            ?: obj.stringOrNull("src")
            ?: return null

        if (url.isBlank()) return null

        // Extract ID
        val identifier = obj.stringOrNull("id")
            ?: obj.stringOrNull("identifier")
            ?: obj.stringOrNull("idf")
            ?: obj.intOrNull("id")?.toString()

        // Extract alt text
        val alt = obj.stringOrNull("alt")
            ?: obj.stringOrNull("altText")
            ?: obj.stringOrNull("alt_text")
            ?: obj.stringOrNull("description")
            ?: obj.stringOrNull("title")

        // Extract weather station ID
        val weatherStationId = obj.intOrNull("weatherStationId")

        return MediaItem(
            url = url,
            type = detectMediaType(url),
            identifier = identifier,
            alt = alt,
            weatherStationId = weatherStationId,
        )
    }

    private fun detectMediaType(url: String): MediaType {
        val embedUrl = YouTubeUrlHelper.extractEmbedUrl(url)
        return if (embedUrl != null) {
            MediaType.YouTubeVideo(embedUrl)
        } else {
            MediaType.Image
        }
    }
}

private fun JsonObject.stringOrNull(key: String): String? {
    val element = this[key] ?: return null
    return if (element is JsonPrimitive && element.isString) element.content else null
}

private fun JsonObject.intOrNull(key: String): Int? {
    val element = this[key] ?: return null
    return if (element is JsonPrimitive) element.intOrNull else null
}
