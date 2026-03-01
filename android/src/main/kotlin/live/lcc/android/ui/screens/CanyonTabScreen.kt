package live.lcc.android.ui.screens

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import live.lcc.android.data.model.MediaItem
import live.lcc.android.data.model.RoadCondition
import live.lcc.android.data.model.WeatherStation
import live.lcc.android.ui.components.EmptyStateView
import live.lcc.android.ui.components.InitialLoadingView
import live.lcc.android.ui.components.MediaCell
import live.lcc.android.ui.components.RoadConditionsBanner

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CanyonTabScreen(
    mediaItems: List<MediaItem>,
    roadConditions: List<RoadCondition>,
    weatherStations: Map<String, WeatherStation>,
    isLoading: Boolean,
    isRefreshing: Boolean,
    imageRevision: Long,
    gridColumns: Int,
    error: String?,
    onItemClick: (index: Int) -> Unit,
    onRefresh: () -> Unit,
    modifier: Modifier = Modifier,
    imageUrlBuilder: (MediaItem, Long) -> String,
) {
    PullToRefreshBox(
        isRefreshing = isRefreshing,
        onRefresh = onRefresh,
        modifier = modifier,
    ) {
        when {
            isLoading && mediaItems.isEmpty() -> {
                InitialLoadingView(columns = gridColumns)
            }
            error != null && mediaItems.isEmpty() -> {
                EmptyStateView(
                    message = error,
                    onRetry = onRefresh,
                )
            }
            mediaItems.isEmpty() -> {
                EmptyStateView(message = "No cameras available")
            }
            else -> {
                LazyVerticalGrid(
                    columns = GridCells.Fixed(gridColumns),
                    contentPadding = PaddingValues(2.dp),
                    horizontalArrangement = Arrangement.spacedBy(2.dp),
                    verticalArrangement = Arrangement.spacedBy(2.dp),
                ) {
                    // Road conditions banner (full-width)
                    if (roadConditions.isNotEmpty()) {
                        item(span = { GridItemSpan(maxLineSpan) }) {
                            RoadConditionsBanner(conditions = roadConditions)
                        }
                    }

                    // Camera grid
                    itemsIndexed(
                        items = mediaItems,
                        key = { _, item -> item.identifier ?: item.id.toString() },
                    ) { index, item ->
                        val station = if (gridColumns == 1) {
                            item.identifier?.let { id -> weatherStations[id] }
                        } else {
                            null
                        }
                        MediaCell(
                            item = item,
                            imageUrl = imageUrlBuilder(item, imageRevision),
                            weatherStation = station,
                            onClick = { onItemClick(index) },
                        )
                    }
                }
            }
        }
    }
}
