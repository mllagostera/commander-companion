package com.commandercompanion.presentation.screens.register

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.remote.api.AuthApi
import com.commandercompanion.data.remote.dto.RegisterRequest
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

data class RegisterUiState(
    val isLoading: Boolean = false,
    val error: String? = null,
    /** No-null una vez que el registro terminó bien: dispara la pantalla de "revisá tu email". */
    val registeredEmail: String? = null
)

/**
 * Registro por email/password contra `POST /auth/register`. A diferencia de [login][
 * com.commandercompanion.presentation.screens.login.LoginViewModel.loginWithPassword], un
 * registro exitoso NO deja sesión iniciada (el backend no devuelve tokens, solo el usuario
 * creado) — mismo contrato que el cliente web (`web/app/composables/useAuth.ts`), así que la
 * pantalla muestra "revisá tu email" en vez de navegar directo al dashboard.
 */
@HiltViewModel
class RegisterViewModel @Inject constructor(
    private val authApi: AuthApi
) : ViewModel() {

    private val _uiState = MutableStateFlow(RegisterUiState())
    val uiState: StateFlow<RegisterUiState> = _uiState.asStateFlow()

    fun register(username: String, email: String, password: String) {
        if (username.isBlank() || email.isBlank() || password.isBlank()) {
            _uiState.update { it.copy(error = "Completá todos los campos") }
            return
        }
        if (password.length < MIN_PASSWORD_LENGTH) {
            _uiState.update { it.copy(error = "La contraseña debe tener al menos $MIN_PASSWORD_LENGTH caracteres") }
            return
        }
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                authApi.register(RegisterRequest(username.trim(), email.trim(), password))
                _uiState.update { it.copy(isLoading = false, registeredEmail = email.trim()) }
            } catch (e: HttpException) {
                _uiState.update { it.copy(isLoading = false, error = mapRegisterError(e)) }
            } catch (e: IOException) {
                _uiState.update { it.copy(isLoading = false, error = "No se pudo conectar con el servidor") }
            }
        }
    }

    private fun mapRegisterError(e: HttpException): String = when (e.code()) {
        409 -> "Ya existe una cuenta con ese email o nombre de usuario"
        400 -> "Revisá los datos ingresados"
        else -> "No se pudo crear la cuenta (error ${e.code()})"
    }
}
