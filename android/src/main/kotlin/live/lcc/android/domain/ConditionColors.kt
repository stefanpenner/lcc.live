package live.lcc.android.domain

import androidx.compose.ui.graphics.Color
import live.lcc.android.ui.theme.Amber500
import live.lcc.android.ui.theme.Blue500
import live.lcc.android.ui.theme.Green500
import live.lcc.android.ui.theme.Indigo500
import live.lcc.android.ui.theme.LightBlue400
import live.lcc.android.ui.theme.Purple500
import live.lcc.android.ui.theme.Red500
import live.lcc.android.ui.theme.Slate500

object ConditionColors {

    fun roadConditionColor(condition: String): Color {
        return when (condition.lowercase().trim()) {
            "dry" -> Green500
            "wet" -> Blue500
            "snowy", "snow covered" -> LightBlue400
            "icy", "ice" -> Indigo500
            "slush" -> Purple500
            else -> Slate500
        }
    }

    fun weatherConditionColor(condition: String): Color {
        return when (condition.lowercase().trim()) {
            "fair", "clear" -> Amber500
            "cloudy", "overcast" -> Slate500
            "rain", "rainy" -> Blue500
            "snow", "snowy", "snowing" -> LightBlue400
            "fog", "foggy" -> Slate500
            "hail" -> Red500
            else -> Slate500
        }
    }

    fun restrictionColor(restriction: String): Color {
        return when {
            restriction.lowercase().contains("closed") -> Red500
            restriction.lowercase().contains("chain") -> Amber500
            restriction.lowercase().contains("traction") -> Amber500
            restriction.isBlank() || restriction.lowercase() == "none" -> Green500
            else -> Amber500
        }
    }
}
