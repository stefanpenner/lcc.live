package live.lcc.android.ui.screens

import android.content.Intent
import android.net.Uri
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectVerticalDragGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Share
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import live.lcc.android.data.model.MediaItem
import live.lcc.android.data.model.MediaType
import live.lcc.android.data.model.WeatherStation
import live.lcc.android.domain.GalleryHelpers
import live.lcc.android.ui.components.WeatherChips
import live.lcc.android.ui.components.ZoomableImage
import kotlin.math.abs

@Composable
fun GalleryScreen(
    mediaItems: List<MediaItem>,
    weatherStations: Map<String, WeatherStation>,
    initialIndex: Int,
    imageRevision: Long,
    onDismiss: () -> Unit,
) {
    val pagerState = rememberPagerState(
        initialPage = initialIndex.coerceIn(0, (mediaItems.size - 1).coerceAtLeast(0)),
        pageCount = { mediaItems.size },
    )

    var dragOffsetY by remember { mutableFloatStateOf(0f) }
    val dismissThreshold = 200f
    val velocityThreshold = 800f

    val backgroundAlpha by animateFloatAsState(
        targetValue = (1f - (abs(dragOffsetY) / 400f)).coerceIn(0.3f, 1f),
        animationSpec = spring(stiffness = Spring.StiffnessLow),
        label = "bg_alpha",
    )

    val context = LocalContext.current

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = backgroundAlpha))
            .systemBarsPadding(),
    ) {
        // Pager with drag-dismiss
        HorizontalPager(
            state = pagerState,
            modifier = Modifier
                .fillMaxSize()
                .graphicsLayer {
                    translationY = dragOffsetY
                    alpha = backgroundAlpha
                }
                .pointerInput(Unit) {
                    detectVerticalDragGestures(
                        onDragEnd = {
                            if (abs(dragOffsetY) > dismissThreshold) {
                                onDismiss()
                            } else {
                                dragOffsetY = 0f
                            }
                        },
                        onDragCancel = { dragOffsetY = 0f },
                        onVerticalDrag = { _, dragAmount ->
                            // Rubber-band effect beyond threshold
                            val multiplier = if (abs(dragOffsetY) > dismissThreshold) 0.3f else 1f
                            dragOffsetY += dragAmount * multiplier
                        },
                    )
                },
        ) { page ->
            val item = mediaItems[page]
            when (val type = item.type) {
                is MediaType.YouTubeVideo -> {
                    Box(
                        modifier = Modifier.fillMaxSize(),
                        contentAlignment = Alignment.Center,
                    ) {
                        val watchUrl = type.embedURL.replace("/embed/", "/watch?v=")
                        IconButton(onClick = {
                            context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(watchUrl)))
                        }) {
                            Icon(
                                imageVector = Icons.Default.PlayArrow,
                                contentDescription = "Play on YouTube",
                                tint = Color.White,
                                modifier = Modifier.padding(16.dp),
                            )
                        }
                    }
                }
                is MediaType.Image -> {
                    val separator = if ('?' in item.url) '&' else '?'
                    val url = "${item.url}${separator}r=$imageRevision"
                    ZoomableImage(
                        imageUrl = url,
                        contentDescription = item.alt,
                    )
                }
            }
        }

        // Top bar with close and share
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(8.dp)
                .align(Alignment.TopStart),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(onClick = onDismiss) {
                Icon(
                    imageVector = Icons.Default.Close,
                    contentDescription = "Close",
                    tint = Color.White,
                )
            }
            Spacer(modifier = Modifier.weight(1f))
            Text(
                text = "${pagerState.currentPage + 1} of ${mediaItems.size}",
                color = Color.White,
                style = MaterialTheme.typography.bodyMedium,
            )
            Spacer(modifier = Modifier.weight(1f))
            IconButton(onClick = {
                val item = mediaItems[pagerState.currentPage]
                val shareUrl = GalleryHelpers.shareUrl(item)
                val sendIntent = Intent().apply {
                    action = Intent.ACTION_SEND
                    putExtra(Intent.EXTRA_TEXT, shareUrl)
                    type = "text/plain"
                }
                context.startActivity(Intent.createChooser(sendIntent, null))
            }) {
                Icon(
                    imageVector = Icons.Default.Share,
                    contentDescription = "Share",
                    tint = Color.White,
                )
            }
        }

        // Bottom caption and weather overlay
        val currentItem = mediaItems.getOrNull(pagerState.currentPage)
        currentItem?.let { item ->
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .align(Alignment.BottomStart)
                    .background(Color.Black.copy(alpha = 0.5f))
                    .padding(16.dp),
            ) {
                item.caption?.let { caption ->
                    Text(
                        text = caption,
                        color = Color.White,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
                // Weather info for this camera
                item.identifier?.let { id ->
                    weatherStations[id]?.let { station ->
                        Spacer(modifier = Modifier.height(4.dp))
                        WeatherChips(station = station)
                    }
                }
            }
        }
    }
}
