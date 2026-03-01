package live.lcc.android.di

import live.lcc.android.data.remote.LccApiClient
import live.lcc.android.data.repository.CanyonRepository
import live.lcc.android.data.repository.WeatherRepository
import live.lcc.android.service.ImagePollingService
import live.lcc.android.service.MetricsService
import live.lcc.android.service.NetworkMonitor
import live.lcc.android.ui.screens.CanyonViewModel
import okhttp3.OkHttpClient
import org.koin.android.ext.koin.androidContext
import org.koin.core.module.dsl.viewModel
import org.koin.dsl.module
import java.util.concurrent.TimeUnit

val appModule = module {
    // Networking
    single {
        OkHttpClient.Builder()
            .connectTimeout(30, TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .build()
    }
    single { LccApiClient(get()) }

    // Services
    single { NetworkMonitor(androidContext()) }
    single { MetricsService(get()) }
    single { ImagePollingService() }

    // Repositories
    single { CanyonRepository(get()) }
    single { WeatherRepository(get()) }

    // ViewModels
    viewModel { CanyonViewModel(get(), get(), get(), get()) }
}
