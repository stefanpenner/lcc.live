package live.lcc.android.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import live.lcc.android.data.model.WeatherStation

@Composable
fun WeatherChips(
    station: WeatherStation,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        station.airTempInt?.let { temp ->
            WeatherChip(text = "${temp}\u00B0F", color = tempColor(temp))
        }
        station.surfaceStatus?.let { status ->
            if (status.isNotBlank() && status.lowercase() != "null") {
                WeatherChip(text = status)
            }
        }
        station.precipitation?.let { precip ->
            if (precip.isNotBlank() && precip != "0" && precip.lowercase() != "null") {
                WeatherChip(text = "\u2744 $precip")
            }
        }
        station.windSpeedAvgInt?.let { wind ->
            if (wind > 0) {
                WeatherChip(text = "\uD83D\uDCA8 ${wind}mph")
            }
        }
    }
}

/** Overlay-style weather chips for on-image display (like iOS MediaCell overlay). */
@Composable
fun OverlayWeatherChips(
    station: WeatherStation,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(3.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        station.airTempInt?.let { temp ->
            OverlayChip(text = "${temp}\u00B0F")
        }
        station.surfaceStatus?.let { status ->
            if (status.isNotBlank() && status.lowercase() != "null") {
                OverlayChip(text = status)
            }
        }
        station.precipitation?.let { precip ->
            if (precip.isNotBlank() && precip != "0" && precip.lowercase() != "null") {
                OverlayChip(text = "\u2744 $precip")
            }
        }
        station.windSpeedAvgInt?.let { wind ->
            if (wind > 0) {
                OverlayChip(text = "${wind}mph")
            }
        }
    }
}

/** Single overlay chip: white text on semi-transparent white bg, matching iOS style. */
@Composable
private fun OverlayChip(
    text: String,
    modifier: Modifier = Modifier,
) {
    Text(
        text = text,
        modifier = modifier
            .clip(RoundedCornerShape(4.dp))
            .background(Color.White.copy(alpha = 0.12f))
            .padding(horizontal = 4.dp, vertical = 1.dp),
        color = Color.White.copy(alpha = 0.75f),
        fontSize = 10.sp,
        fontWeight = FontWeight.Medium,
        maxLines = 1,
    )
}

@Composable
fun WeatherChip(
    text: String,
    color: Color = MaterialTheme.colorScheme.primary,
    modifier: Modifier = Modifier,
) {
    Text(
        text = text,
        modifier = modifier
            .clip(RoundedCornerShape(4.dp))
            .background(color.copy(alpha = 0.15f))
            .padding(horizontal = 6.dp, vertical = 2.dp),
        color = color,
        fontSize = 10.sp,
        fontWeight = FontWeight.Bold,
        maxLines = 1,
    )
}

private fun tempColor(temp: Int): Color {
    return when {
        temp <= 20 -> Color(0xFF60a5fa) // light blue - very cold
        temp <= 32 -> Color(0xFF3b82f6) // blue - freezing
        temp <= 50 -> Color(0xFF10b981) // green - cool
        temp <= 70 -> Color(0xFFf59e0b) // amber - mild
        else -> Color(0xFFef4444) // red - hot
    }
}
