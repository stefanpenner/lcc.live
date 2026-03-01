package live.lcc.android.domain

import live.lcc.android.data.model.MediaItem
import live.lcc.android.data.remote.LccApiClient

object GalleryHelpers {

    fun shareUrl(item: MediaItem, baseUrl: String = LccApiClient.DEFAULT_BASE_URL): String {
        val slug = item.slug
        return if (!slug.isNullOrEmpty()) {
            "$baseUrl/camera/$slug"
        } else {
            baseUrl
        }
    }

    fun adjacentIndices(currentIndex: Int, total: Int): List<Int> {
        if (total <= 1) return emptyList()
        val indices = mutableListOf<Int>()
        if (currentIndex > 0) indices.add(currentIndex - 1)
        if (currentIndex < total - 1) indices.add(currentIndex + 1)
        return indices
    }
}
