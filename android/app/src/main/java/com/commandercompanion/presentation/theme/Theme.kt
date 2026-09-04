package com.commandercompanion.presentation.theme

import android.app.Activity
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.SideEffect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat

private val DarkColorScheme = darkColorScheme(
    primary = PrimaryAccent,
    onPrimary = AppBackgroundDeep,
    secondary = SecondaryAccent,
    onSecondary = AppBackgroundDeep,
    background = AppBackground,
    onBackground = AppOnBackground,
    surface = AppSurface,
    onSurface = AppOnBackground,
    surfaceVariant = AppSurfaceVariant,
    onSurfaceVariant = AppOnSurfaceVariant,
    outline = AppOutline,
    error = StatusDanger,
    onError = AppBackgroundDeep,
    errorContainer = Color(0x33F87171),
    onErrorContainer = StatusDanger
)

// We force dark theme for Commander Companion by default as it's better for battery and eyes during long games.
@Composable
fun CommanderCompanionTheme(
    darkTheme: Boolean = true,
    content: @Composable () -> Unit
) {
    val colorScheme = DarkColorScheme
    val view = LocalView.current
    if (!view.isInEditMode) {
        SideEffect {
            // The window is edge-to-edge (see MainActivity), so neither bar has a colour to set --
            // both are transparent and the app paints under them. All that is left is the icon
            // tint, which has to be driven from here because it follows the composable's
            // darkTheme argument rather than anything declared in themes.xml.
            val window = (view.context as Activity).window
            val insetsController = WindowCompat.getInsetsController(window, view)
            insetsController.isAppearanceLightStatusBars = !darkTheme
            insetsController.isAppearanceLightNavigationBars = !darkTheme
        }
    }

    MaterialTheme(
        colorScheme = colorScheme,
        typography = Typography,
        content = content
    )
}
