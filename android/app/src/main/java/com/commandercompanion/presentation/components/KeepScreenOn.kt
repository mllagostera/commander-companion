package com.commandercompanion.presentation.components

import android.view.View
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.ui.platform.LocalView
import java.util.WeakHashMap

/**
 * Screens currently asking each host view to stay awake. Every screen shares the Activity's
 * single ComposeView, and during a navigation transition the incoming screen's effect runs
 * before the outgoing one is disposed -- a plain on/off flag would let the pre-game screen
 * switch the flag off right after the tracker switched it on. Counting holders avoids that.
 * Only touched from the main thread (composition effects), so no synchronisation is needed.
 */
private val screenOnHolders = WeakHashMap<View, Int>()

/** Keeps the display from timing out while the calling composable is in the composition. */
@Composable
fun KeepScreenOn() {
    val view = LocalView.current
    DisposableEffect(view) {
        screenOnHolders[view] = (screenOnHolders[view] ?: 0) + 1
        view.keepScreenOn = true
        onDispose {
            val remaining = (screenOnHolders[view] ?: 1) - 1
            if (remaining <= 0) {
                screenOnHolders.remove(view)
                view.keepScreenOn = false
            } else {
                screenOnHolders[view] = remaining
            }
        }
    }
}
