package live.lcc.android.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil.compose.AsyncImage
import live.lcc.android.data.model.MediaItem
import live.lcc.android.data.model.MediaType
import live.lcc.android.data.model.WeatherStation
import live.lcc.android.domain.YouTubeUrlHelper

@Composable
fun MediaCell(
    item: MediaItem,
    imageUrl: String,
    weatherStation: WeatherStation?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .aspectRatio(16f / 14.4f)
            .clip(RoundedCornerShape(6.dp))
            .clickable(onClick = onClick),
    ) {
        val displayUrl = when (val type = item.type) {
            is MediaType.YouTubeVideo -> {
                YouTubeUrlHelper.thumbnailUrl(type.embedURL) ?: imageUrl
            }
            else -> imageUrl
        }

        // Image fills the entire cell
        AsyncImage(
            model = displayUrl,
            contentDescription = item.alt,
            modifier = Modifier.fillMaxSize(),
            contentScale = ContentScale.Crop,
        )

        // YouTube play overlay
        if (item.isYouTube) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(Color.Black.copy(alpha = 0.3f)),
                contentAlignment = Alignment.Center,
            ) {
                Box(
                    modifier = Modifier
                        .size(48.dp)
                        .background(Color.Black.copy(alpha = 0.7f), CircleShape),
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(
                        imageVector = Icons.Default.PlayArrow,
                        contentDescription = "Play",
                        tint = Color.White,
                        modifier = Modifier.size(32.dp),
                    )
                }
            }
        }

        // Bottom gradient overlay with caption + weather (like iOS)
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .align(Alignment.BottomStart)
                .background(
                    Brush.verticalGradient(
                        colors = listOf(
                            Color.Transparent,
                            Color.Black.copy(alpha = 0.65f),
                        ),
                    ),
                )
                .padding(horizontal = 6.dp, vertical = 4.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                // Camera name
                item.alt?.let { alt ->
                    Text(
                        text = alt,
                        color = Color.White,
                        fontSize = 10.sp,
                        fontWeight = FontWeight.Medium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f, fill = false),
                    )
                }

                // Weather chips inline
                weatherStation?.let { station ->
                    Spacer(modifier = Modifier.width(2.dp))
                    OverlayWeatherChips(station = station)
                }
            }
        }
    }
}
