package com.commandercompanion.presentation.screens.settings

import androidx.appcompat.app.AppCompatDelegate
import androidx.core.os.LocaleListCompat
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.remote.api.AuthApi
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.ChangePasswordRequest
import com.commandercompanion.data.remote.dto.UpdateProfileRequest
import com.commandercompanion.data.remote.dto.UserDto
import com.commandercompanion.data.session.SessionManager
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import retrofit2.HttpException
import java.io.IOException
import javax.inject.Inject

private const val MIN_PASSWORD_LENGTH = 8

data class SettingsUiState(
    val isLoadingProfile: Boolean = true,
    val user: UserDto? = null,
    val loadError: String? = null,

    val isSavingUsername: Boolean = false,
    val usernameError: String? = null,

    val isSavingMoxfieldUsername: Boolean = false,
    val moxfieldUsernameError: String? = null,

    val isChangingPassword: Boolean = false,
    val passwordError: String? = null,
    val passwordChanged: Boolean = false,

    val language: AppLanguage = AppLanguage.SPANISH
)

/**
 * Own account settings: edit username, link/edit the Moxfield username, and change
 * password — same scope as `web/app/pages/settings.vue`, against the same endpoints
 * (`PATCH /users/{id}`, `POST /users/{id}/password`). Doesn't include the bulk Moxfield import
 * (still behind a flag/broken on the web, see `docs/roadmap/TASKS.md`).
 *
 * `PATCH /users/{id}` doesn't carry the `Bearer` as an explicit header (unlike [AuthApi.me]):
 * it goes through [CommanderApi]'s authenticated client, which already attaches it via
 * `AuthInterceptor`. We only need our own `userId`, resolved once when the profile loads via `AuthApi.me`.
 */
@HiltViewModel
class SettingsViewModel @Inject constructor(
    private val authApi: AuthApi,
    private val commanderApi: CommanderApi,
    private val sessionManager: SessionManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(
        SettingsUiState(language = AppLanguage.fromTag(AppCompatDelegate.getApplicationLocales().toLanguageTags()))
    )
    val uiState: StateFlow<SettingsUiState> = _uiState.asStateFlow()

    init {
        loadProfile()
    }

    private fun loadProfile() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoadingProfile = true, loadError = null) }
            try {
                val token = sessionManager.currentAccessToken()
                    ?: throw IllegalStateException("no hay sesión activa")
                val user = authApi.me("Bearer $token")
                _uiState.update { it.copy(isLoadingProfile = false, user = user) }
            } catch (e: Exception) {
                _uiState.update { it.copy(isLoadingProfile = false, loadError = "No se pudo cargar tu perfil") }
            }
        }
    }

    fun updateUsername(username: String) {
        val userId = _uiState.value.user?.id ?: return
        if (username.isBlank()) {
            _uiState.update { it.copy(usernameError = "El nombre de usuario no puede estar vacío") }
            return
        }
        viewModelScope.launch {
            _uiState.update { it.copy(isSavingUsername = true, usernameError = null) }
            try {
                val updated = commanderApi.updateProfile(userId, UpdateProfileRequest(username = username.trim()))
                _uiState.update { it.copy(isSavingUsername = false, user = updated) }
            } catch (e: HttpException) {
                _uiState.update { it.copy(isSavingUsername = false, usernameError = mapUsernameError(e)) }
            } catch (e: IOException) {
                _uiState.update { it.copy(isSavingUsername = false, usernameError = "No se pudo conectar con el servidor") }
            }
        }
    }

    fun updateMoxfieldUsername(moxfieldUsername: String) {
        val userId = _uiState.value.user?.id ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isSavingMoxfieldUsername = true, moxfieldUsernameError = null) }
            try {
                val updated = commanderApi.updateProfile(
                    userId,
                    UpdateProfileRequest(moxfieldUsername = moxfieldUsername.trim())
                )
                _uiState.update { it.copy(isSavingMoxfieldUsername = false, user = updated) }
            } catch (e: HttpException) {
                _uiState.update {
                    it.copy(isSavingMoxfieldUsername = false, moxfieldUsernameError = "No se pudo guardar el usuario de Moxfield")
                }
            } catch (e: IOException) {
                _uiState.update {
                    it.copy(isSavingMoxfieldUsername = false, moxfieldUsernameError = "No se pudo conectar con el servidor")
                }
            }
        }
    }

    fun changePassword(currentPassword: String, newPassword: String, newPasswordConfirm: String) {
        val userId = _uiState.value.user?.id ?: return
        if (newPassword != newPasswordConfirm) {
            _uiState.update { it.copy(passwordError = "Las contraseñas nuevas no coinciden") }
            return
        }
        if (newPassword.length < MIN_PASSWORD_LENGTH) {
            _uiState.update { it.copy(passwordError = "La contraseña debe tener al menos $MIN_PASSWORD_LENGTH caracteres") }
            return
        }
        viewModelScope.launch {
            _uiState.update { it.copy(isChangingPassword = true, passwordError = null, passwordChanged = false) }
            try {
                commanderApi.changePassword(userId, ChangePasswordRequest(currentPassword, newPassword))
                _uiState.update { it.copy(isChangingPassword = false, passwordChanged = true) }
            } catch (e: HttpException) {
                _uiState.update { it.copy(isChangingPassword = false, passwordError = mapChangePasswordError(e)) }
            } catch (e: IOException) {
                _uiState.update { it.copy(isChangingPassword = false, passwordError = "No se pudo conectar con el servidor") }
            }
        }
    }

    /**
     * Overrides the app's language regardless of the device's own setting, persisted
     * automatically across restarts (`autoStoreLocales`, see `AndroidManifest.xml`) — same 3
     * languages the web client offers (`values`/`values-en`/`values-ca`). Setting it recreates
     * every active `Activity` so the new resources take effect immediately; the callback-free
     * `AppCompatDelegate` API works with a plain `ComponentActivity`, no `AppCompatActivity`
     * required (AppCompat 1.6.0+).
     */
    fun changeLanguage(language: AppLanguage) {
        _uiState.update { it.copy(language = language) }
        AppCompatDelegate.setApplicationLocales(LocaleListCompat.forLanguageTags(language.tag))
    }

    fun logout(onComplete: () -> Unit) {
        viewModelScope.launch {
            sessionManager.logout()
            onComplete()
        }
    }

    private fun mapUsernameError(e: HttpException): String = when (e.code()) {
        409 -> "Ese nombre de usuario ya está en uso"
        400 -> "El nombre de usuario no puede estar vacío"
        else -> "No se pudo guardar el nombre de usuario (error ${e.code()})"
    }

    private fun mapChangePasswordError(e: HttpException): String = when (e.code()) {
        400 -> "La contraseña nueva debe tener al menos $MIN_PASSWORD_LENGTH caracteres"
        401 -> "La contraseña actual no es correcta"
        else -> "No se pudo cambiar la contraseña (error ${e.code()})"
    }
}
