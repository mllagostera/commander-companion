package com.vansid.tapeandocartones.presentation.theme

import androidx.compose.ui.graphics.Brush

/** Gradient used for primary CTAs ("NUEVA PARTIDA", "Iniciar sesión", etc). */
val AccentGradient: Brush
    get() = Brush.horizontalGradient(listOf(AccentVioletStart, AccentVioletEnd))

/** Gradient used for headline text (app name, "TapeandoCartones"). */
val TitleGradient: Brush
    get() = Brush.horizontalGradient(listOf(AccentSoft, AccentSofter))
