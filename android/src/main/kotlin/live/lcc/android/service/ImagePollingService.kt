package live.lcc.android.service

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import live.lcc.android.data.model.MediaItem
import live.lcc.android.data.model.MediaType
import timber.log.Timber

/**
 * Periodically invalidates image cache to trigger Coil reloads.
 * Increments a revision counter that composables observe to reload images.
 */
class ImagePollingService {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    private val _imageRevision = MutableStateFlow(0L)
    val imageRevision: StateFlow<Long> = _imageRevision.asStateFlow()

    companion object {
        private const val POLL_INTERVAL_MS = 5_000L
    }

    var isActive: Boolean = true

    fun start() {
        scope.launch {
            while (true) {
                delay(POLL_INTERVAL_MS)
                if (isActive) {
                    _imageRevision.value++
                    Timber.v("Image revision: ${_imageRevision.value}")
                }
            }
        }
    }

    /**
     * Build a cache-busting URL for a media item based on the current revision.
     */
    fun imageUrlWithRevision(item: MediaItem, revision: Long): String {
        if (item.type != MediaType.Image) return item.url
        val separator = if ('?' in item.url) '&' else '?'
        return "${item.url}${separator}r=$revision"
    }
}
