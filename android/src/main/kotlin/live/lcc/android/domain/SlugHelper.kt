package live.lcc.android.domain

import live.lcc.android.data.model.slugify

object SlugHelper {
    fun slugify(text: String): String = live.lcc.android.data.model.slugify(text)

    fun cameraShareUrl(baseUrl: String, alt: String): String {
        val slug = slugify(alt)
        return if (slug.isNotEmpty()) "$baseUrl/camera/$slug" else baseUrl
    }
}
