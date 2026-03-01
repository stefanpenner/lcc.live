package live.lcc.android.ui.components

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.expandVertically
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.ui.graphics.Color
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import live.lcc.android.data.model.RoadCondition
import live.lcc.android.domain.ConditionColors

@Composable
fun RoadConditionsBanner(
    conditions: List<RoadCondition>,
    modifier: Modifier = Modifier,
) {
    AnimatedVisibility(
        visible = conditions.isNotEmpty(),
        enter = expandVertically(),
        exit = shrinkVertically(),
    ) {
        Column(
            modifier = modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(6.dp))
                .background(Color(0xFF1A1A1A))
                .padding(horizontal = 10.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            conditions.forEach { condition ->
                RoadConditionRow(condition)
            }
        }
    }
}

@Composable
private fun RoadConditionRow(condition: RoadCondition) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        // Status dot
        val dotColor = ConditionColors.roadConditionColor(condition.roadCondition)
        androidx.compose.foundation.Canvas(
            modifier = Modifier.size(8.dp),
        ) {
            drawCircle(color = dotColor)
        }

        Column(modifier = Modifier.weight(1f)) {
            Row(
                horizontalArrangement = Arrangement.spacedBy(6.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = condition.roadwayName,
                    color = Color.White,
                    fontSize = 13.sp,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                if (condition.roadCondition.isNotBlank()) {
                    WeatherChip(
                        text = condition.roadCondition,
                        color = dotColor,
                    )
                }
                if (condition.weatherCondition.isNotBlank()) {
                    WeatherChip(
                        text = condition.weatherCondition,
                        color = ConditionColors.weatherConditionColor(condition.weatherCondition),
                    )
                }
            }

            if (condition.restriction.isNotBlank() && condition.restriction.lowercase() != "none") {
                Text(
                    text = condition.restriction,
                    color = ConditionColors.restrictionColor(condition.restriction),
                    fontSize = 11.sp,
                )
            }
        }

        Text(
            text = condition.timeAgo(),
            color = Color.White.copy(alpha = 0.35f),
            fontSize = 10.sp,
        )
    }
}
