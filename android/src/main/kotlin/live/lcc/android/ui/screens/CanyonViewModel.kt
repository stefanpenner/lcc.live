package live.lcc.android.ui.screens

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import live.lcc.android.data.repository.CanyonRepository
import live.lcc.android.data.repository.WeatherRepository
import live.lcc.android.service.ImagePollingService
import live.lcc.android.service.NetworkMonitor

class CanyonViewModel(
    private val canyonRepository: CanyonRepository,
    private val weatherRepository: WeatherRepository,
    private val networkMonitor: NetworkMonitor,
    private val imagePollingService: ImagePollingService,
) : ViewModel() {

    // Canyon data
    val lccMediaItems = canyonRepository.lccMediaItems
    val bccMediaItems = canyonRepository.bccMediaItems
    val lccRoadConditions = canyonRepository.lccRoadConditions
    val bccRoadConditions = canyonRepository.bccRoadConditions
    val lccWeatherStations = canyonRepository.lccWeatherStations
    val bccWeatherStations = canyonRepository.bccWeatherStations
    val isLoading = canyonRepository.isLoading
    val lccError = canyonRepository.lccError
    val bccError = canyonRepository.bccError

    // Network state
    val connectionState = networkMonitor.connectionState
    val isOnline = networkMonitor.isOnline

    // Image polling
    val imageRevision = imagePollingService.imageRevision
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), 0L)

    // UI state
    private val _selectedTab = MutableStateFlow(CanyonTab.LCC)
    val selectedTab: StateFlow<CanyonTab> = _selectedTab.asStateFlow()

    private val _gridColumns = MutableStateFlow(2)
    val gridColumns: StateFlow<Int> = _gridColumns.asStateFlow()

    private val _isRefreshing = MutableStateFlow(false)
    val isRefreshing: StateFlow<Boolean> = _isRefreshing.asStateFlow()

    companion object {
        private const val POLL_INTERVAL_MS = 5_000L
    }

    init {
        networkMonitor.start()
        imagePollingService.start()
        loadInitialData()
        startPolling()
    }

    private fun loadInitialData() {
        viewModelScope.launch {
            launch { canyonRepository.refreshLcc() }
            launch { canyonRepository.refreshBcc() }
            launch { canyonRepository.refreshUdot("LCC") }
            launch { canyonRepository.refreshUdot("BCC") }
        }
    }

    private fun startPolling() {
        viewModelScope.launch {
            while (true) {
                delay(POLL_INTERVAL_MS)
                if (networkMonitor.isOnline.value) {
                    launch { canyonRepository.refreshLcc() }
                    launch { canyonRepository.refreshBcc() }
                    launch { canyonRepository.refreshUdot("LCC") }
                    launch { canyonRepository.refreshUdot("BCC") }
                }
            }
        }
    }

    fun selectTab(tab: CanyonTab) {
        _selectedTab.value = tab
    }

    fun toggleGridColumns() {
        _gridColumns.value = if (_gridColumns.value == 2) 1 else 2
    }

    fun refresh() {
        viewModelScope.launch {
            _isRefreshing.value = true
            launch { canyonRepository.refreshLcc() }
            launch { canyonRepository.refreshBcc() }
            launch { canyonRepository.refreshUdot("LCC") }
            launch { canyonRepository.refreshUdot("BCC") }
            delay(500) // brief minimum refresh indicator
            _isRefreshing.value = false
        }
    }

    override fun onCleared() {
        super.onCleared()
        networkMonitor.stop()
    }
}

enum class CanyonTab(val label: String, val canyonId: String) {
    LCC("LCC", "LCC"),
    BCC("BCC", "BCC"),
}
