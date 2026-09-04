package com.commandercompanion

import android.graphics.Color
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import com.commandercompanion.presentation.navigation.AppNavigation
import com.commandercompanion.presentation.theme.AppBackground
import com.commandercompanion.presentation.theme.CommanderCompanionTheme
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        // Edge-to-edge: the app paints the entire window, status bar and gesture bar included,
        // so the system no longer reserves (and colours) a strip of its own at either end -- which
        // is what used to leave a pale band under the gesture handle on a near-black screen.
        // Both bars get a fully transparent scrim instead of the default auto-scrim: every screen
        // sits on AppScreenBackground's near-black gradient, so the light icons already have
        // contrast. SystemBarStyle.dark() is what asks for those light icons.
        // This is also the only supported behaviour from targetSdk 35 on, where Android 15 ignores
        // statusBarColor/navigationBarColor altogether.
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(Color.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(Color.TRANSPARENT)
        )
        super.onCreate(savedInstanceState)
        setContent {
            CommanderCompanionTheme {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = AppBackground
                ) {
                    AppNavigation()
                }
            }
        }
    }
}
