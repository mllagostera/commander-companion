package com.commandercompanion

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import com.commandercompanion.presentation.navigation.AppNavigation
import com.commandercompanion.presentation.theme.CommanderCompanionTheme
import com.commandercompanion.presentation.theme.MdThemeDarkBackground
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            CommanderCompanionTheme {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MdThemeDarkBackground
                ) {
                    AppNavigation()
                }
            }
        }
    }
}
