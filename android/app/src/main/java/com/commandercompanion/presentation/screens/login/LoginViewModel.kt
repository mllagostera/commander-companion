package com.commandercompanion.presentation.screens.login

import android.content.Context
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.remote.api.AuthApi
import com.commandercompanion.data.remote.dto.GoogleLoginRequest
import com.commandercompanion.data.remote.dto.LoginRequest
import com.commandercompanion.data.session.GoogleAuthClient
import com.commandercompanion.data.session.GoogleSignInCancelledException
import com.commandercompanion.data.session.NoGoogleAccountException
import com.commandercompanion.data.session.SessionManager
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
    val loginSucceeded: Boolean = false
)

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
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val response = authApi.login(LoginRequest(email.trim(), password))
                sessionManager.saveSession(response)
                _uiState.update { it.copy(isLoading = false, loginSucceeded = true) }
            } catch (e: HttpException) {
                _uiState.update { it.copy(isLoading = false, error = mapPasswordError(e)) }
            } catch (e: IOException) {
                _uiState.update { it.copy(isLoading = false, error = LoginError.Network) }
            }
        }
    }

    /** [context] must be an Activity context: Credential Manager needs to be able to show UI. */
    fun loginWithGoogle(context: Context) {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
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

    private fun mapPasswordError(e: HttpException): LoginError = when (e.code()) {
        401 -> LoginError.BadCredentials
        else -> LoginError.Unknown(e.code())
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
