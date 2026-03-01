package live.lcc.android

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import live.lcc.android.ui.screens.MainScreen
import live.lcc.android.ui.theme.LccTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            LccTheme {
                MainScreen()
            }
        }
    }
}
