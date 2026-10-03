package com.vansid.tapeandocartones.presentation.screens.forgotpassword

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
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
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import com.vansid.tapeandocartones.R
import com.vansid.tapeandocartones.presentation.components.AppLogoMark
import com.vansid.tapeandocartones.presentation.components.AppScreenBackground
import com.vansid.tapeandocartones.presentation.components.AuthTextField
import com.vansid.tapeandocartones.presentation.components.GlassCard
import com.vansid.tapeandocartones.presentation.components.GradientButton
import com.vansid.tapeandocartones.presentation.components.GradientTitle
import com.vansid.tapeandocartones.presentation.theme.AccentSoft
import com.vansid.tapeandocartones.presentation.theme.AppFaint
import com.vansid.tapeandocartones.presentation.theme.AppOnBackground

/** Maps the ViewModel's error type onto the translated strings (see [ForgotPasswordError]). */
@Composable
private fun ForgotPasswordError.message(): String = when (this) {
    ForgotPasswordError.EmptyEmail -> stringResource(R.string.error_forgot_password_empty_email)
    ForgotPasswordError.InvalidEmail -> stringResource(R.string.error_register_invalid_email)
    ForgotPasswordError.Network -> stringResource(R.string.error_api_network)
    ForgotPasswordError.TooManyRequests -> stringResource(R.string.error_forgot_password_too_many_requests)
    is ForgotPasswordError.Unknown -> stringResource(R.string.error_forgot_password_unknown, code)
}

/**
 * "Forgot your password?": asks for a reset link (see [ForgotPasswordViewModel]). [initialEmail]
 * is whatever the user had typed on the login screen, so they don't have to type it again.
 */
@Composable
fun ForgotPasswordScreen(
    initialEmail: String,
    onNavigateToLogin: () -> Unit,
    viewModel: ForgotPasswordViewModel = hiltViewModel()
) {
    var email by rememberSaveable { mutableStateOf(initialEmail) }
    val uiState by viewModel.uiState.collectAsState()
    // The language the strings are actually shown in (see app_locale_tag), so the reset email
    // matches it.
    val uiLanguage = stringResource(R.string.app_locale_tag)

    AppScreenBackground {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            AppLogoMark()
            Spacer(modifier = Modifier.height(14.dp))

            val sentTo = uiState.sentTo
            if (sentTo != null) {
                ResetLinkSent(email = sentTo, onNavigateToLogin = onNavigateToLogin)
            } else {
                GradientTitle(text = stringResource(R.string.forgot_password_title), fontSize = 18.sp)
                Spacer(modifier = Modifier.height(6.dp))
                Text(
                    text = stringResource(R.string.forgot_password_intro),
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    fontSize = 13.sp,
                    textAlign = TextAlign.Center
                )
                Spacer(modifier = Modifier.height(28.dp))

                GlassCard(
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(28.dp),
                    contentPadding = PaddingValues(24.dp)
                ) {
                    Column(modifier = Modifier.fillMaxWidth()) {
                        AuthTextField(
                            label = stringResource(R.string.common_email_label),
                            value = email,
                            onValueChange = { email = it },
                            enabled = !uiState.isLoading,
                            keyboardType = KeyboardType.Email
                        )

                        uiState.error?.let { error ->
                            Spacer(modifier = Modifier.height(10.dp))
                            Text(
                                text = error.message(),
                                color = MaterialTheme.colorScheme.error,
                                fontSize = 12.sp
                            )
                        }

                        Spacer(modifier = Modifier.height(14.dp))
                        GradientButton(
                            text = stringResource(R.string.forgot_password_submit),
                            onClick = { viewModel.requestReset(email, uiLanguage) },
                            enabled = !uiState.isLoading
                        ) {
                            if (uiState.isLoading) {
                                CircularProgressIndicator(
                                    modifier = Modifier.size(18.dp),
                                    strokeWidth = 2.dp,
                                    color = MaterialTheme.colorScheme.background
                                )
                            } else {
                                Text(
                                    stringResource(R.string.forgot_password_submit),
                                    color = MaterialTheme.colorScheme.background,
                                    fontWeight = FontWeight.SemiBold,
                                    fontSize = 13.sp
                                )
                            }
                        }

                        Spacer(modifier = Modifier.height(14.dp))
                        Text(
                            text = stringResource(R.string.forgot_password_back_to_login),
                            fontSize = 12.sp,
                            color = AccentSoft,
                            fontWeight = FontWeight.SemiBold,
                            textAlign = TextAlign.Center,
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable(enabled = !uiState.isLoading, onClick = onNavigateToLogin)
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun ResetLinkSent(email: String, onNavigateToLogin: () -> Unit) {
    GradientTitle(text = stringResource(R.string.forgot_password_sent_title), fontSize = 18.sp)
    Spacer(modifier = Modifier.height(24.dp))
    GlassCard(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(28.dp),
        contentPadding = PaddingValues(24.dp)
    ) {
        Column(
            modifier = Modifier.fillMaxWidth(),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Text(
                text = stringResource(R.string.forgot_password_sent_intro),
                color = AppFaint,
                fontSize = 13.sp,
                textAlign = TextAlign.Center
            )
            Text(
                text = email,
                color = AppOnBackground,
                fontWeight = FontWeight.SemiBold,
                fontSize = 13.sp,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = stringResource(R.string.forgot_password_sent_outro),
                color = AppFaint,
                fontSize = 13.sp,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(18.dp))
            GradientButton(
                text = stringResource(R.string.forgot_password_back_to_login),
                onClick = onNavigateToLogin
            )
        }
    }
}
