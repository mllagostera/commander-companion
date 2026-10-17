package com.vansid.tapeandocartones

import android.content.pm.PackageManager
import android.graphics.Color
import android.os.Build
import android.os.Bundle
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import androidx.core.content.ContextCompat
import com.vansid.tapeandocartones.core.util.isLocalNetworkUrl
import com.vansid.tapeandocartones.presentation.navigation.AppNavigation
import com.vansid.tapeandocartones.presentation.theme.AppBackground
import com.vansid.tapeandocartones.presentation.theme.TapeandoCartonesTheme
import dagger.hilt.android.AndroidEntryPoint

/**
 * [AppCompatActivity] rather than a plain `ComponentActivity`, and not for any of AppCompat's UI:
 * `AppCompatDelegate.setApplicationLocales()` (the language selector in Settings) can only reach
 * the platform `LocaleManager` through a Context that AppCompat itself has captured, and the only
 * thing that captures it is an AppCompatActivity being created. With a plain ComponentActivity the
 * call returned without applying anything and `getApplicationLocales()` stayed empty, so picking a
 * language did nothing. This also requires an AppCompat parent theme -- see `themes.xml`.
 */
@AndroidEntryPoint
class MainActivity : AppCompatActivity() {

    // The outcome needs no handling: denied, the requests to the backend fail like any other
    // network error, and the permission can still be granted from the system settings.
    private val localNetworkPermission =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) { }

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
        if (savedInstanceState == null) requestLocalNetworkForDevBackend()
        setContent {
            TapeandoCartonesTheme {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = AppBackground
                ) {
                    AppNavigation()
                }
            }
        }
    }

    /**
     * Android 17 blocks traffic to local network addresses (10.x, 192.168.x, ...) for apps
     * targeting API 37 unless `ACCESS_LOCAL_NETWORK` is granted. Only a debug build pointed at a
     * development backend needs that -- the emulator's 10.0.2.2, or a LAN IP for a phone -- so
     * the permission is declared in the debug manifest alone and asked for only in that case.
     * Release builds talk to the deployed backend over the internet and never ask.
     */
    private fun requestLocalNetworkForDevBackend() {
        if (!BuildConfig.DEBUG || Build.VERSION.SDK_INT < LOCAL_NETWORK_PERMISSION_SDK) return
        if (!isLocalNetworkUrl(BuildConfig.API_BASE_URL)) return
        if (ContextCompat.checkSelfPermission(this, ACCESS_LOCAL_NETWORK) == PackageManager.PERMISSION_GRANTED) {
            return
        }
        localNetworkPermission.launch(ACCESS_LOCAL_NETWORK)
    }

    private companion object {
        /** Android 17, the first release that enforces the local network permission. */
        const val LOCAL_NETWORK_PERMISSION_SDK = 37
        const val ACCESS_LOCAL_NETWORK = "android.permission.ACCESS_LOCAL_NETWORK"
    }
}
