package live.lcc.android.ui.screens

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Apps
import androidx.compose.material.icons.filled.Landscape
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import live.lcc.android.data.model.MediaItem
import live.lcc.android.data.model.MediaType
import live.lcc.android.data.model.RoadCondition
import live.lcc.android.data.model.WeatherStation
import live.lcc.android.domain.ConnectionStatusHelper
import live.lcc.android.ui.components.ConnectionStatusSheet
import org.koin.androidx.compose.koinViewModel

@Composable
fun MainScreen(
    viewModel: CanyonViewModel = koinViewModel(),
) {
    val selectedTab by viewModel.selectedTab.collectAsState()
    val gridColumns by viewModel.gridColumns.collectAsState()
    val isRefreshing by viewModel.isRefreshing.collectAsState()
    val connectionState by viewModel.connectionState.collectAsState()
    val imageRevision by viewModel.imageRevision.collectAsState()

    val lccItems by viewModel.lccMediaItems.collectAsState()
    val lccRoadConditions by viewModel.lccRoadConditions.collectAsState()
    val lccWeatherStations by viewModel.lccWeatherStations.collectAsState()
    val lccError by viewModel.lccError.collectAsState()
    val isLoading by viewModel.isLoading.collectAsState()

    val bccItems by viewModel.bccMediaItems.collectAsState()
    val bccRoadConditions by viewModel.bccRoadConditions.collectAsState()
    val bccWeatherStations by viewModel.bccWeatherStations.collectAsState()
    val bccError by viewModel.bccError.collectAsState()

    val items: List<MediaItem>
    val roadConditions: List<RoadCondition>
    val weatherStations: Map<String, WeatherStation>
    val error: String?
    when (selectedTab) {
        CanyonTab.LCC -> {
            items = lccItems
            roadConditions = lccRoadConditions
            weatherStations = lccWeatherStations
            error = lccError
        }
        CanyonTab.BCC -> {
            items = bccItems
            roadConditions = bccRoadConditions
            weatherStations = bccWeatherStations
            error = bccError
        }
    }

    var galleryIndex by remember { mutableStateOf<Int?>(null) }
    var showConnectionSheet by remember { mutableStateOf(false) }
    val context = LocalContext.current

    Scaffold(
        containerColor = Color.Black,
        bottomBar = {
            NavigationBar(
                containerColor = Color(0xFF0A0A0A),
                tonalElevation = 0.dp,
            ) {
                // Connection dot on the far left
                IconButton(
                    onClick = { showConnectionSheet = true },
                    modifier = Modifier.weight(0.15f),
                ) {
                    val color = ConnectionStatusHelper.color(connectionState)
                    Canvas(modifier = Modifier.size(10.dp)) {
                        drawCircle(color = color)
                    }
                }

                // Canyon tabs in the center
                CanyonTab.entries.forEach { tab ->
                    NavigationBarItem(
                        selected = selectedTab == tab,
                        onClick = { viewModel.selectTab(tab) },
                        icon = {
                            Icon(
                                imageVector = Icons.Default.Landscape,
                                contentDescription = tab.label,
                                modifier = Modifier.size(22.dp),
                            )
                        },
                        label = {
                            Text(
                                text = tab.label,
                                fontSize = 11.sp,
                                fontWeight = if (selectedTab == tab) FontWeight.Bold else FontWeight.Normal,
                            )
                        },
                        colors = NavigationBarItemDefaults.colors(
                            selectedIconColor = Color.White,
                            selectedTextColor = Color.White,
                            unselectedIconColor = Color.White.copy(alpha = 0.4f),
                            unselectedTextColor = Color.White.copy(alpha = 0.4f),
                            indicatorColor = Color.White.copy(alpha = 0.1f),
                        ),
                    )
                }

                // Grid toggle on the far right
                IconButton(
                    onClick = { viewModel.toggleGridColumns() },
                    modifier = Modifier.weight(0.15f),
                ) {
                    Icon(
                        imageVector = Icons.Default.Apps,
                        contentDescription = "Toggle grid size",
                        tint = Color.White.copy(alpha = 0.6f),
                        modifier = Modifier.size(20.dp),
                    )
                }
            }
        },
    ) { innerPadding ->
        CanyonTabScreen(
            mediaItems = items,
            roadConditions = roadConditions,
            weatherStations = weatherStations,
            isLoading = isLoading,
            isRefreshing = isRefreshing,
            imageRevision = imageRevision,
            gridColumns = gridColumns,
            error = error,
            onItemClick = { index ->
                when (val type = items[index].type) {
                    is MediaType.YouTubeVideo ->
                        context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(type.watchUrl())))
                    else -> galleryIndex = index
                }
            },
            onRefresh = { viewModel.refresh() },
            modifier = Modifier
                .fillMaxSize()
                .background(Color.Black)
                .padding(innerPadding),
            imageUrlBuilder = { item, revision ->
                val baseUrl = item.url
                val separator = if ('?' in baseUrl) '&' else '?'
                "$baseUrl${separator}r=$revision"
            },
        )
    }

    galleryIndex?.let { index ->
        if (items.isNotEmpty()) {
            GalleryScreen(
                mediaItems = items,
                weatherStations = weatherStations,
                initialIndex = index,
                imageRevision = imageRevision,
                onDismiss = { galleryIndex = null },
            )
        }
    }

    if (showConnectionSheet) {
        ConnectionStatusSheet(
            connectionState = connectionState,
            onDismiss = { showConnectionSheet = false },
        )
    }
}

private fun MediaType.YouTubeVideo.watchUrl(): String =
    embedURL.replace("/embed/", "/watch?v=")
