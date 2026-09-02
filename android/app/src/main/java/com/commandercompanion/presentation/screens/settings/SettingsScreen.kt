package com.commandercompanion.presentation.screens.settings

import android.graphics.Bitmap
import android.graphics.Color as AndroidColor
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.FilterQuality
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import com.commandercompanion.BuildConfig
import com.commandercompanion.R
import com.commandercompanion.core.util.QrEncoder
import com.commandercompanion.core.util.friendQrLink
import com.google.zxing.common.BitMatrix
import com.commandercompanion.presentation.components.AppScreenBackground
import com.commandercompanion.presentation.components.AuthTextField
import com.commandercompanion.presentation.components.CircleIconButton
import com.commandercompanion.presentation.components.GlassCard
import com.commandercompanion.presentation.components.GradientButton
import com.commandercompanion.presentation.components.PillSegmentedControl
import com.commandercompanion.presentation.components.SectionEyebrow
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppOnBackground
import com.commandercompanion.presentation.theme.StatusDanger

@Composable
fun SettingsScreen(
    onBack: () -> Unit,
    onLoggedOut: () -> Unit,
    viewModel: SettingsViewModel = hiltViewModel()
) {
    val state by viewModel.uiState.collectAsState()

    AppScreenBackground {
        Column(modifier = Modifier.fillMaxSize().padding(20.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                CircleIconButton(label = "‹", onClick = onBack)
                Text(
                    stringResource(R.string.settings_title),
                    color = AppOnBackground,
                    fontSize = 17.sp
                )
            }
            Spacer(modifier = Modifier.height(16.dp))

            when {
                state.isLoadingProfile -> Column(
                    modifier = Modifier.fillMaxSize(),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center
                ) { CircularProgressIndicator() }

                state.loadError != null -> Text(state.loadError!!, color = StatusDanger, fontSize = 13.sp)

                state.user != null -> Column(
                    modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()),
                    verticalArrangement = Arrangement.spacedBy(16.dp)
                ) {
                    ProfileSection(state = state, onSaveUsername = viewModel::updateUsername)
                    FriendQrSection(userId = state.user!!.id)
                    LanguageSection(state = state, onSelect = viewModel::changeLanguage)
                    MoxfieldSection(state = state, onSave = viewModel::updateMoxfieldUsername)
                    SecuritySection(
                        state = state,
                        onChangePassword = viewModel::changePassword,
                        onLogout = { viewModel.logout(onLoggedOut) }
                    )
                }
            }
        }
    }
}

/**
 * The user's own profile QR — the same card the web client shows in its
 * settings, encoding the same `/friends/add/{id}` link so either client can
 * scan the other's code (see [friendQrLink]).
 *
 * The matrix is [remember]ed on the id: encoding is cheap but not free, and
 * without it every recomposition of the settings screen would redo it.
 */
@Composable
private fun FriendQrSection(userId: String) {
    val link = remember(userId) { friendQrLink(BuildConfig.WEB_APP_URL, userId) }
    val qr = remember(link) { QrEncoder.encode(link).toImageBitmap() }

    GlassCard(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(20.dp)) {
        Column(modifier = Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            SectionEyebrow(stringResource(R.string.settings_qr_heading))
            Text(stringResource(R.string.settings_qr_body), color = AppFaint, fontSize = 12.sp)
            Image(
                bitmap = qr,
                // The QR is not decorative, but nothing a screen reader can do
                // with the image itself helps: the description says what it is.
                contentDescription = stringResource(R.string.settings_qr_content_description),
                filterQuality = FilterQuality.None,
                modifier = Modifier
                    .align(Alignment.CenterHorizontally)
                    .size(220.dp)
                    .background(Color.White, RoundedCornerShape(12.dp))
                    .padding(10.dp)
            )
        }
    }
}

/**
 * Draws the matrix into a bitmap, one pixel per module.
 *
 * `FilterQuality.None` at the call site is what keeps it crisp: the image is
 * scaled up to 220dp from a matrix of a few dozen modules, and any smoothing
 * blurs the edges enough to hurt scanning.
 *
 * This is the one part of QR generation that can't be unit tested — Bitmap is
 * an Android framework class with no JVM implementation — which is exactly why
 * everything else lives in [QrEncoder] instead of here.
 */
private fun BitMatrix.toImageBitmap(): ImageBitmap {
    val pixels = IntArray(width * height)
    for (y in 0 until height) {
        for (x in 0 until width) {
            pixels[y * width + x] = if (this[x, y]) AndroidColor.BLACK else AndroidColor.WHITE
        }
    }
    return Bitmap.createBitmap(pixels, width, height, Bitmap.Config.ARGB_8888).asImageBitmap()
}

@Composable
private fun ProfileSection(state: SettingsUiState, onSaveUsername: (String) -> Unit) {
    var username by remember(state.user?.id) { mutableStateOf(state.user?.username.orEmpty()) }

    GlassCard(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(20.dp)) {
        Column(modifier = Modifier.fillMaxWidth()) {
            SectionEyebrow(stringResource(R.string.settings_profile_heading))
            Spacer(modifier = Modifier.height(4.dp))
            Text(state.user?.email.orEmpty(), color = AppFaint, fontSize = 12.sp)
            Spacer(modifier = Modifier.height(12.dp))
            AuthTextField(
                label = stringResource(R.string.setup_name_label),
                value = username,
                onValueChange = { username = it },
                enabled = !state.isSavingUsername
            )
            state.usernameError?.let { message ->
                Spacer(modifier = Modifier.height(6.dp))
                Text(message, color = MaterialTheme.colorScheme.error, fontSize = 12.sp)
            }
            Spacer(modifier = Modifier.height(10.dp))
            GradientButton(
                text = if (state.isSavingUsername) {
                    stringResource(R.string.common_saving)
                } else {
                    stringResource(R.string.common_save)
                },
                onClick = { onSaveUsername(username) },
                enabled = !state.isSavingUsername
            )
        }
    }
}

@Composable
private fun LanguageSection(state: SettingsUiState, onSelect: (AppLanguage) -> Unit) {
    GlassCard(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(20.dp)) {
        Column(modifier = Modifier.fillMaxWidth()) {
            SectionEyebrow(stringResource(R.string.settings_language_heading))
            Spacer(modifier = Modifier.height(12.dp))
            PillSegmentedControl(
                options = AppLanguage.entries.map { it to stringResource(it.labelRes) },
                selected = state.language,
                onSelected = onSelect
            )
        }
    }
}

@Composable
private fun MoxfieldSection(state: SettingsUiState, onSave: (String) -> Unit) {
    var moxfieldUsername by remember(state.user?.id) { mutableStateOf(state.user?.moxfieldUsername.orEmpty()) }

    GlassCard(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(20.dp)) {
        Column(modifier = Modifier.fillMaxWidth()) {
            SectionEyebrow(stringResource(R.string.settings_moxfield_heading))
            Spacer(modifier = Modifier.height(4.dp))
            Text(stringResource(R.string.settings_moxfield_description), color = AppFaint, fontSize = 12.sp)
            Spacer(modifier = Modifier.height(12.dp))
            AuthTextField(
                label = stringResource(R.string.settings_moxfield_username_label),
                value = moxfieldUsername,
                onValueChange = { moxfieldUsername = it },
                enabled = !state.isSavingMoxfieldUsername
            )
            state.moxfieldUsernameError?.let { message ->
                Spacer(modifier = Modifier.height(6.dp))
                Text(message, color = MaterialTheme.colorScheme.error, fontSize = 12.sp)
            }
            Spacer(modifier = Modifier.height(10.dp))
            GradientButton(
                text = if (state.isSavingMoxfieldUsername) {
                    stringResource(R.string.common_saving)
                } else {
                    stringResource(R.string.common_save)
                },
                onClick = { onSave(moxfieldUsername) },
                enabled = !state.isSavingMoxfieldUsername
            )
        }
    }
}

@Composable
private fun SecuritySection(
    state: SettingsUiState,
    onChangePassword: (current: String, new: String, confirm: String) -> Unit,
    onLogout: () -> Unit
) {
    var currentPassword by remember { mutableStateOf("") }
    var newPassword by remember { mutableStateOf("") }
    var newPasswordConfirm by remember { mutableStateOf("") }

    LaunchedEffect(state.passwordChanged) {
        if (state.passwordChanged) {
            currentPassword = ""
            newPassword = ""
            newPasswordConfirm = ""
        }
    }

    GlassCard(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(20.dp)) {
        Column(modifier = Modifier.fillMaxWidth()) {
            SectionEyebrow(stringResource(R.string.settings_security_heading))
            Spacer(modifier = Modifier.height(12.dp))

            if (state.user?.hasPassword == true) {
                AuthTextField(
                    label = stringResource(R.string.settings_security_current_password),
                    value = currentPassword,
                    onValueChange = { currentPassword = it },
                    enabled = !state.isChangingPassword,
                    visualTransformation = PasswordVisualTransformation()
                )
                Spacer(modifier = Modifier.height(10.dp))
                AuthTextField(
                    label = stringResource(R.string.settings_security_new_password),
                    value = newPassword,
                    onValueChange = { newPassword = it },
                    enabled = !state.isChangingPassword,
                    visualTransformation = PasswordVisualTransformation()
                )
                Spacer(modifier = Modifier.height(10.dp))
                AuthTextField(
                    label = stringResource(R.string.settings_security_confirm_password),
                    value = newPasswordConfirm,
                    onValueChange = { newPasswordConfirm = it },
                    enabled = !state.isChangingPassword,
                    visualTransformation = PasswordVisualTransformation()
                )
                state.passwordError?.let { message ->
                    Spacer(modifier = Modifier.height(6.dp))
                    Text(message, color = MaterialTheme.colorScheme.error, fontSize = 12.sp)
                }
                if (state.passwordChanged) {
                    Spacer(modifier = Modifier.height(6.dp))
                    Text(
                        stringResource(R.string.settings_security_password_updated),
                        color = MaterialTheme.colorScheme.primary,
                        fontSize = 12.sp
                    )
                }
                Spacer(modifier = Modifier.height(10.dp))
                GradientButton(
                    text = if (state.isChangingPassword) {
                        stringResource(R.string.common_saving)
                    } else {
                        stringResource(R.string.settings_security_submit)
                    },
                    onClick = { onChangePassword(currentPassword, newPassword, newPasswordConfirm) },
                    enabled = !state.isChangingPassword
                )
            } else {
                Text(stringResource(R.string.settings_security_no_password), color = AppFaint, fontSize = 12.sp)
            }

            Spacer(modifier = Modifier.height(16.dp))
            GradientButton(text = stringResource(R.string.dashboard_logout), onClick = onLogout)
        }
    }
}
