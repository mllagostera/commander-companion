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
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import retrofit2.HttpException
import java.io.IOException
import javax.inject.Inject

data class LoginUiState(
    val isLoading: Boolean = false,
    val error: String? = null,
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
            _uiState.update { it.copy(error = "Completá email y contraseña") }
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
                _uiState.update { it.copy(isLoading = false, error = "No se pudo conectar con el servidor") }
            }
        }
    }

    /** [context] debe ser un contexto de Activity: Credential Manager necesita poder mostrar UI. */
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
            _uiState.update { it.copy(isLoading = false, error = "No se pudo conectar con el servidor") }
        }
    }

    fun errorShown() {
        _uiState.update { it.copy(error = null) }
    }

    private fun mapPasswordError(e: HttpException): String = when (e.code()) {
        401 -> "Email o contraseña incorrectos"
        else -> "No se pudo iniciar sesión (error ${e.code()})"
    }

    private fun mapGoogleBackendError(e: HttpException): String = when (e.code()) {
        400 -> "Google rechazó la cuenta (email no verificado o token inválido)"
        501 -> "El inicio de sesión con Google no está configurado en el servidor todavía"
        else -> "No se pudo iniciar sesión con Google (error ${e.code()})"
    }

    private fun mapGoogleSignInError(throwable: Throwable): String? = when (throwable) {
        // El usuario cerró el selector de cuentas: no es un error, no mostramos banner.
        is GoogleSignInCancelledException -> null
        is NoGoogleAccountException -> throwable.message
        else -> "No se pudo iniciar sesión con Google"
    }
}
