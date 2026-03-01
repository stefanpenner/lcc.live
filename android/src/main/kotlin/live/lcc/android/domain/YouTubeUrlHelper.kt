package live.lcc.android.domain

object YouTubeUrlHelper {

    private val EMBED_REGEX = Regex("""youtube\.com/embed/([a-zA-Z0-9_-]+)""")
    private val WATCH_REGEX = Regex("""youtube\.com/watch\?.*v=([a-zA-Z0-9_-]+)""")
    private val SHORT_REGEX = Regex("""youtu\.be/([a-zA-Z0-9_-]+)""")

    fun extractVideoId(url: String): String? {
        EMBED_REGEX.find(url)?.groupValues?.get(1)?.let { return it }
        WATCH_REGEX.find(url)?.groupValues?.get(1)?.let { return it }
        SHORT_REGEX.find(url)?.groupValues?.get(1)?.let { return it }
        return null
    }

    fun extractEmbedUrl(url: String): String? {
        val videoId = extractVideoId(url) ?: return null
        return "https://www.youtube.com/embed/$videoId"
    }

    fun thumbnailUrl(url: String): String? {
        val videoId = extractVideoId(url) ?: return null
        return "https://img.youtube.com/vi/$videoId/maxresdefault.jpg"
    }

    fun isYouTubeUrl(url: String): Boolean {
        return extractVideoId(url) != null
    }
}
