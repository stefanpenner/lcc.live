package live.lcc.android

import android.app.Application
import android.content.pm.ApplicationInfo
import live.lcc.android.di.appModule
import org.koin.android.ext.koin.androidContext
import org.koin.android.ext.koin.androidLogger
import org.koin.core.context.startKoin
import timber.log.Timber

class LccApplication : Application() {
    override fun onCreate() {
        super.onCreate()

        val isDebug = applicationInfo.flags and ApplicationInfo.FLAG_DEBUGGABLE != 0
        if (isDebug) {
            Timber.plant(Timber.DebugTree())
        }

        startKoin {
            androidLogger()
            androidContext(this@LccApplication)
            modules(appModule)
        }
    }
}
