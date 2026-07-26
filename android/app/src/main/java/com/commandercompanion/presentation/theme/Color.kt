package com.commandercompanion.presentation.theme

import androidx.compose.ui.graphics.Color

// Base Colors
val MdThemeDarkBackground = Color(0xFF121212)
val MdThemeDarkSurface = Color(0xFF1E1E1E)
val MdThemeDarkOnBackground = Color(0xFFE0E0E0)
val MdThemeDarkOnSurface = Color(0xFFE0E0E0)

// MTG Mana Inspired Colors
val ManaWhite = Color(0xFFF8E7B9)
val ManaBlue = Color(0xFFB3CEEA)
val ManaBlack = Color(0xFFA69F9D)
val ManaRed = Color(0xFFEB9F82)
val ManaGreen = Color(0xFFC4D3CA)
val ManaColorless = Color(0xFFC9C9C9)

// Primary/Secondary accents based on a premium dark feel
val PrimaryAccent = ManaWhite
val SecondaryAccent = ManaRed

// Selectable palette for player identification (WUBRG + colorless)
val PlayerColorPalette: List<Pair<String, Color>> = listOf(
    "white" to ManaWhite,
    "blue" to ManaBlue,
    "black" to ManaBlack,
    "red" to ManaRed,
    "green" to ManaGreen,
    "colorless" to ManaColorless
)

fun colorForKey(colorKey: String): Color =
    PlayerColorPalette.firstOrNull { it.first == colorKey }?.second ?: ManaColorless
