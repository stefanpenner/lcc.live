package live.lcc.android.domain

import androidx.compose.ui.graphics.Color
import live.lcc.android.ui.theme.Green500
import live.lcc.android.ui.theme.Red500
import live.lcc.android.ui.theme.Amber500

enum class ConnectionState {
    CONNECTED,
    DEGRADED,
    DISCONNECTED,
}

object ConnectionStatusHelper {

    fun color(state: ConnectionState): Color {
        return when (state) {
            ConnectionState.CONNECTED -> Green500
            ConnectionState.DEGRADED -> Amber500
            ConnectionState.DISCONNECTED -> Red500
        }
    }

    fun label(state: ConnectionState): String {
        return when (state) {
            ConnectionState.CONNECTED -> "Connected"
            ConnectionState.DEGRADED -> "Degraded"
            ConnectionState.DISCONNECTED -> "Disconnected"
        }
    }
}
