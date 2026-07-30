package com.commandercompanion.presentation.theme

import androidx.compose.ui.graphics.Color

// Base surface colors — dark violet-tinted, matching the app's card-game brand identity.
val AppBackground = Color(0xFF050308)
val AppBackgroundDeep = Color(0xFF0A0714)
val AppBackgroundGlow = Color(0xFF1E1B4B)
val AppSurface = Color(0xFF15121F)
val AppSurfaceVariant = Color(0xFF1F1B2E)
val AppOnBackground = Color(0xFFF1F0F6)
val AppOnSurfaceVariant = Color(0xFFA5A3B8)
val AppMuted = Color(0xFF8B87A3)
val AppFaint = Color(0xFF726F89)
val AppOutline = Color(0x1FFFFFFF)

// Primary/secondary accents — the violet-to-purple gradient used across buttons and headlines.
val AccentVioletStart = Color(0xFF8B5CF6)
val AccentVioletEnd = Color(0xFFA855F7)
val AccentSoft = Color(0xFFC4B5FD)
val AccentSofter = Color(0xFFDDD6FE)

val StatusSuccess = Color(0xFF5EEAD4)
val StatusSuccessContainer = Color(0x2634D399)
val StatusInfo = AccentSoft
val StatusInfoContainer = Color(0x2EC4B5FD)
val StatusDanger = Color(0xFFF87171)
val StatusPoison = Color(0xFF86EFAC)

// MTG Mana Inspired Colors — kept as the persisted colorKey vocabulary (Room + backend).
val ManaWhite = Color(0xFFF8E7B9)
val ManaBlue = Color(0xFFB3CEEA)
val ManaBlack = Color(0xFFA69F9D)
val ManaRed = Color(0xFFEB9F82)
val ManaGreen = Color(0xFFC4D3CA)
val ManaColorless = Color(0xFFC9C9C9)

// Primary/Secondary accents based on a premium dark feel
val PrimaryAccent = AccentVioletStart
val SecondaryAccent = AccentVioletEnd

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
