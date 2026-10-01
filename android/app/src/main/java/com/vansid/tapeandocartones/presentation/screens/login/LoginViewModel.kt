package com.vansid.tapeandocartones.presentation.screens.login

import android.content.Context
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.vansid.tapeandocartones.data.remote.api.AuthApi
import com.vansid.tapeandocartones.data.remote.dto.GoogleLoginRequest
import com.vansid.tapeandocartones.data.remote.dto.LoginRequest
import com.vansid.tapeandocartones.data.remote.dto.ResendVerificationRequest
import com.vansid.tapeandocartones.data.session.GoogleAuthClient
import com.vansid.tapeandocartones.data.session.GoogleSignInCancelledException
import com.vansid.tapeandocartones.data.session.NoGoogleAccountException
import com.vansid.tapeandocartones.data.session.SessionManager
import dagger.hilt.android.lifecycle.HiltViewModel
import java.io.IOException
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import retrofit2.HttpException

/**
 * What went wrong, for the screen to turn into a string resource — same reasoning as
 * `FriendsError`: a literal here could not be translated into the three locales the app ships.
 * Sealed rather than an enum because the two "unknown" cases carry the status code their
 * message interpolates.
 */
sealed interface LoginError {
    data object EmptyFields : LoginError
    data object Network : LoginError
    data object BadCredentials : LoginError
    data object EmailNotConfirmed : LoginError
    data object AccountDeactivated : LoginError
    data object ResendFailed : LoginError
    data class Unknown(val code: Int) : LoginError
    data object GoogleRejected : LoginError
    data object GoogleNotConfigured : LoginError
    data class GoogleBackend(val code: Int) : LoginError
    data object GoogleNoAccount : LoginError
    data object GoogleUnknown : LoginError
}

data class LoginUiState(
    val isLoading: Boolean = false,
    val error: LoginError? = null,
    val loginSucceeded: Boolean = false,
    /** The last password login hit an unconfirmed email: the screen offers a resend. */
    val needsVerification: Boolean = false,
    val isResending: Boolean = false,
    /** A resend was accepted; replaces the resend link with a "check your inbox". */
    val resendSent: Boolean = false
)

/**
 * Maps a failed `POST /auth/login` to a [LoginError]. Both an unconfirmed email and a
 * deactivated account come back as 403 — the backend checks the email first — so the
 * error message is what tells them apart (see `users.ErrAccountDeactivated`).
 */
internal fun passwordLoginError(code: Int, errorBody: String?): LoginError = when (code) {
    401 -> LoginError.BadCredentials
    403 -> if (errorBody?.contains("deactivated") == true) {
        LoginError.AccountDeactivated
    } else {
        LoginError.EmailNotConfirmed
    }
    else -> LoginError.Unknown(code)
}

@HiltViewModel
class LoginViewModel @Inject constructor(
    private val authApi: AuthApi,
    private val sessionManager: SessionManager,
    private val googleAuthClient: GoogleAuthClient
) : ViewModel() {

    private val _uiState = MutableStateFlow(LoginUiState())
    val uiState: StateFlow<LoginUiState> = _uiState.asStateFlow()

    fun loginWithPassword(email: String, password: String) {
        if (email.isBlank() || password.isBlank()) {
            _uiState.update { it.copy(error = LoginError.EmptyFields) }
            return
        }
        viewModelScope.launch {
            _uiState.update {
                it.copy(isLoading = true, error = null, needsVerification = false, resendSent = false)
            }
            try {
                val response = authApi.login(LoginRequest(email.trim(), password))
                sessionManager.saveSession(response)
                _uiState.update { it.copy(isLoading = false, loginSucceeded = true) }
            } catch (e: HttpException) {
                val error = passwordLoginError(e.code(), e.response()?.errorBody()?.string())
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        error = error,
                        needsVerification = error == LoginError.EmailNotConfirmed
                    )
                }
            } catch (e: IOException) {
                _uiState.update { it.copy(isLoading = false, error = LoginError.Network) }
            }
        }
    }

    /**
     * Asks for a new verification link for [email], in the app's [locale]. Same contract as
     * the web client's login page: offered only after a login hit an unconfirmed email.
     */
    fun resendVerification(email: String, locale: String) {
        if (email.isBlank()) {
            _uiState.update { it.copy(error = LoginError.EmptyFields) }
            return
        }
        viewModelScope.launch {
            _uiState.update { it.copy(isResending = true, error = null) }
            try {
                authApi.resendVerification(ResendVerificationRequest(email.trim(), locale))
                _uiState.update { it.copy(isResending = false, resendSent = true) }
            } catch (e: HttpException) {
                _uiState.update { it.copy(isResending = false, error = LoginError.ResendFailed) }
            } catch (e: IOException) {
                _uiState.update { it.copy(isResending = false, error = LoginError.Network) }
            }
        }
    }

    /** [context] must be an Activity context: Credential Manager needs to be able to show UI. */
    fun loginWithGoogle(context: Context) {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null, needsVerification = false) }
            googleAuthClient.getIdToken(context).fold(
                onSuccess = { idToken -> exchangeGoogleIdToken(idToken) },
                onFailure = { throwable ->
                    _uiState.update {
                        it.copy(isLoading = false, error = mapGoogleSignInError(throwable))
                    }
                }
            )
        }
    }

    private suspend fun exchangeGoogleIdToken(idToken: String) {
        try {
            val response = authApi.loginWithGoogle(GoogleLoginRequest(idToken))
            sessionManager.saveSession(response)
            _uiState.update { it.copy(isLoading = false, loginSucceeded = true) }
        } catch (e: HttpException) {
            _uiState.update { it.copy(isLoading = false, error = mapGoogleBackendError(e)) }
        } catch (e: IOException) {
            _uiState.update { it.copy(isLoading = false, error = LoginError.Network) }
        }
    }

    fun errorShown() {
        _uiState.update { it.copy(error = null) }
    }

    private fun mapGoogleBackendError(e: HttpException): LoginError = when (e.code()) {
        400 -> LoginError.GoogleRejected
        501 -> LoginError.GoogleNotConfigured
        else -> LoginError.GoogleBackend(e.code())
    }

    private fun mapGoogleSignInError(throwable: Throwable): LoginError? = when (throwable) {
        // The user closed the account picker: not an error, we don't show a banner.
        is GoogleSignInCancelledException -> null
        is NoGoogleAccountException -> LoginError.GoogleNoAccount
        // TODO(TASKS.md Stage 4): every other Credential Manager failure still collapses here,
        // swallowing the GetCredentialException that carries the real cause.
        else -> LoginError.GoogleUnknown
    }
}
