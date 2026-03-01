package live.lcc.android.ui.components

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import live.lcc.android.domain.ConnectionState
import live.lcc.android.domain.ConnectionStatusHelper

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ConnectionStatusSheet(
    connectionState: ConnectionState,
    onDismiss: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState()

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Text(
                text = "Connection Status",
                style = MaterialTheme.typography.headlineMedium,
            )

            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                val color = ConnectionStatusHelper.color(connectionState)
                androidx.compose.foundation.Canvas(
                    modifier = Modifier
                        .size(12.dp)
                        .clip(CircleShape),
                ) {
                    drawCircle(color = color)
                }
                Text(
                    text = ConnectionStatusHelper.label(connectionState),
                    style = MaterialTheme.typography.bodyLarge,
                )
            }

            Text(
                text = when (connectionState) {
                    ConnectionState.CONNECTED -> "Camera feeds are updating in real time."
                    ConnectionState.DEGRADED -> "Connection is unstable. Some images may be delayed."
                    ConnectionState.DISCONNECTED -> "No internet connection. Showing cached data."
                },
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}
