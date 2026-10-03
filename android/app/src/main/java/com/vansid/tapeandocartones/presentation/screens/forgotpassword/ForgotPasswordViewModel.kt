package com.vansid.tapeandocartones.presentation.screens.forgotpassword

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.vansid.tapeandocartones.data.remote.api.AuthApi
import com.vansid.tapeandocartones.data.remote.dto.ForgotPasswordRequest
import com.vansid.tapeandocartones.presentation.screens.register.isPlausibleEmail
import dagger.hilt.android.lifecycle.HiltViewModel
import java.io.IOException
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import retrofit2.HttpException

/** What went wrong, for the screen to translate — see `LoginError` for the reasoning. */
sealed interface ForgotPasswordError {
    data object EmptyEmail : ForgotPasswordError
    data object InvalidEmail : ForgotPasswordError
    data object Network : ForgotPasswordError
    data object TooManyRequests : ForgotPasswordError
    data class Unknown(val code: Int) : ForgotPasswordError
}

data class ForgotPasswordUiState(
    val isLoading: Boolean = false,
    val error: ForgotPasswordError? = null,
    /**
     * Non-null once the request is accepted. The backend answers the same whether the email has
     * an account or not, so this only means "if there's an account, a link is on its way".
     */
    val sentTo: String? = null
)

/**
 * Asks `POST /auth/forgot-password` for a reset link (ADR-0022). The new password itself is
 * chosen on the web client's `/reset-password` page, which is where the emailed link points, so
 * the app only needs this first step.
 */
@HiltViewModel
class ForgotPasswordViewModel @Inject constructor(
    private val authApi: AuthApi
) : ViewModel() {

    private val _uiState = MutableStateFlow(ForgotPasswordUiState())
    val uiState: StateFlow<ForgotPasswordUiState> = _uiState.asStateFlow()

    fun requestReset(email: String, locale: String) {
        if (email.isBlank()) {
            _uiState.update { it.copy(error = ForgotPasswordError.EmptyEmail) }
            return
        }
        if (!isPlausibleEmail(email)) {
            _uiState.update { it.copy(error = ForgotPasswordError.InvalidEmail) }
            return
        }
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                authApi.forgotPassword(ForgotPasswordRequest(email.trim(), locale))
                _uiState.update { it.copy(isLoading = false, sentTo = email.trim()) }
            } catch (e: HttpException) {
                val error = if (e.code() == 429) {
                    ForgotPasswordError.TooManyRequests
                } else {
                    ForgotPasswordError.Unknown(e.code())
                }
                _uiState.update { it.copy(isLoading = false, error = error) }
            } catch (e: IOException) {
                _uiState.update { it.copy(isLoading = false, error = ForgotPasswordError.Network) }
            }
        }
    }
}
