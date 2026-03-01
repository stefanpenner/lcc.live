package live.lcc.android.util

import timber.log.Timber

/**
 * Thin wrapper over Timber for structured logging categories.
 * Usage: Logger.network("Request to %s", url)
 */
object Logger {
    fun app(message: String, vararg args: Any?) = Timber.tag("LCC.App").d(message, *args)
    fun network(message: String, vararg args: Any?) = Timber.tag("LCC.Network").d(message, *args)
    fun ui(message: String, vararg args: Any?) = Timber.tag("LCC.UI").d(message, *args)
    fun performance(message: String, vararg args: Any?) = Timber.tag("LCC.Perf").d(message, *args)
    fun imageLoading(message: String, vararg args: Any?) = Timber.tag("LCC.Image").d(message, *args)
    fun metrics(message: String, vararg args: Any?) = Timber.tag("LCC.Metrics").d(message, *args)
}
