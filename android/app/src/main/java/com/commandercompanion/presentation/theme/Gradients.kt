package com.commandercompanion.presentation.theme

import androidx.compose.ui.graphics.Brush

/** Gradient used for primary CTAs ("NUEVA PARTIDA", "Iniciar sesión", etc). */
val AccentGradient: Brush
    get() = Brush.horizontalGradient(listOf(AccentVioletStart, AccentVioletEnd))

/** Gradient used for headline text (app name, "Commander Companion"). */
val TitleGradient: Brush
    get() = Brush.horizontalGradient(listOf(AccentSoft, AccentSofter))

/** Full-bleed screen background: a soft violet glow fading into near-black. */
val ScreenBackgroundGradient: Brush
    get() = Brush.radialGradient(
        colorStops = arrayOf(
            0.0f to AppBackgroundGlow,
            0.45f to AppBackgroundDeep,
            1.0f to AppBackground
        ),
        radius = 1400f
    )
