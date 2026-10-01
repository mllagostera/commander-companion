package com.vansid.tapeandocartones.presentation.screens.register

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.vansid.tapeandocartones.data.remote.api.AuthApi
import com.vansid.tapeandocartones.data.remote.dto.RegisterRequest
import dagger.hilt.android.lifecycle.HiltViewModel
import java.io.IOException
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import retrofit2.HttpException

private const val MIN_PASSWORD_LENGTH = 8

/**
 * Minimal address shape (something@something.tld, one `@`, no whitespace) -- the same pattern the
 * backend's `normalizeEmail` and the `users_email_format` CHECK enforce, so the form never lets
 * through what the API would reject. A bare username typed into the email field is the case it
 * exists for: that is how an account once ended up unable to log in.
 */
private val EMAIL_SHAPE = Regex("""^[^@\s]+@[^@\s]+\.[^@\s]+$""")

internal fun isPlausibleEmail(email: String): Boolean = EMAIL_SHAPE.matches(email.trim())

/** What went wrong, for the screen to translate — see `LoginError` for the reasoning. */
sealed interface RegisterError {
    data object EmptyFields : RegisterError
    data object InvalidEmail : RegisterError
    data class PasswordTooShort(val minLength: Int) : RegisterError
    data object Network : RegisterError
    data object AlreadyExists : RegisterError
    data object InvalidData : RegisterError
    data class Unknown(val code: Int) : RegisterError
}

data class RegisterUiState(
    val isLoading: Boolean = false,
    val error: RegisterError? = null,
    /** Non-null once registration succeeds: triggers the "check your email" screen. */
    val registeredEmail: String? = null
)

/**
 * Email/password registration against `POST /auth/register`. Unlike [login][
 * com.vansid.tapeandocartones.presentation.screens.login.LoginViewModel.loginWithPassword], a
 * successful registration does NOT leave a session started (the backend returns no tokens, only
 * the created user) — same contract as the web client (`web/app/composables/useAuth.ts`), so the
 * screen shows "check your email" instead of navigating straight to the dashboard.
 */
@HiltViewModel
class RegisterViewModel @Inject constructor(
    private val authApi: AuthApi
) : ViewModel() {

    private val _uiState = MutableStateFlow(RegisterUiState())
    val uiState: StateFlow<RegisterUiState> = _uiState.asStateFlow()

    fun register(username: String, email: String, password: String, locale: String) {
        if (username.isBlank() || email.isBlank() || password.isBlank()) {
            _uiState.update { it.copy(error = RegisterError.EmptyFields) }
            return
        }
        if (!isPlausibleEmail(email)) {
            _uiState.update { it.copy(error = RegisterError.InvalidEmail) }
            return
        }
        if (password.length < MIN_PASSWORD_LENGTH) {
            _uiState.update { it.copy(error = RegisterError.PasswordTooShort(MIN_PASSWORD_LENGTH)) }
            return
        }
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                authApi.register(RegisterRequest(username.trim(), email.trim(), password, locale))
                _uiState.update { it.copy(isLoading = false, registeredEmail = email.trim()) }
            } catch (e: HttpException) {
                _uiState.update { it.copy(isLoading = false, error = mapRegisterError(e)) }
            } catch (e: IOException) {
                _uiState.update { it.copy(isLoading = false, error = RegisterError.Network) }
            }
        }
    }

    private fun mapRegisterError(e: HttpException): RegisterError = when (e.code()) {
        409 -> RegisterError.AlreadyExists
        400 -> RegisterError.InvalidData
        else -> RegisterError.Unknown(e.code())
    }
}
