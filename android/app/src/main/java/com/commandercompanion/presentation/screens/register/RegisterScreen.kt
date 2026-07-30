package com.commandercompanion.presentation.screens.register

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import com.commandercompanion.R
import com.commandercompanion.presentation.components.AppLogoMark
import com.commandercompanion.presentation.components.AppScreenBackground
import com.commandercompanion.presentation.components.AuthTextField
import com.commandercompanion.presentation.components.GlassCard
import com.commandercompanion.presentation.components.GradientButton
import com.commandercompanion.presentation.components.GradientOutlineButton
import com.commandercompanion.presentation.components.GradientTitle
import com.commandercompanion.presentation.components.ThinDivider
import com.commandercompanion.presentation.screens.login.LoginViewModel
import com.commandercompanion.presentation.theme.AccentSoft
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppOnBackground

/**
 * Registro por email/password contra `POST /auth/register` (ver [RegisterViewModel]), más
 * "Continuar con Google" — que reutiliza [LoginViewModel.loginWithGoogle] tal cual, porque para
 * el backend autenticar y registrar por Google es la misma operación (`GoogleLogin` en
 * `backend/internal/auth/service.go` crea la cuenta si no existe).
 *
 * Un registro por password exitoso NO deja sesión iniciada (mismo contrato que el cliente web):
 * en vez de navegar al dashboard, la propia pantalla pasa a mostrar "revisá tu email".
 */
@Composable
fun RegisterScreen(
    onLoginSuccess: () -> Unit,
    onNavigateToLogin: () -> Unit,
    registerViewModel: RegisterViewModel = hiltViewModel(),
    googleViewModel: LoginViewModel = hiltViewModel()
) {
    var username by remember { mutableStateOf("") }
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    val uiState by registerViewModel.uiState.collectAsState()
    val googleUiState by googleViewModel.uiState.collectAsState()
    val context = LocalContext.current

    LaunchedEffect(googleUiState.loginSucceeded) {
        if (googleUiState.loginSucceeded) onLoginSuccess()
    }

    val isBusy = uiState.isLoading || googleUiState.isLoading

    AppScreenBackground {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            AppLogoMark()
            Spacer(modifier = Modifier.height(14.dp))

            if (uiState.registeredEmail != null) {
                RegisteredConfirmation(email = uiState.registeredEmail!!, onNavigateToLogin = onNavigateToLogin)
            } else {
                GradientTitle(text = stringResource(R.string.register_title), fontSize = 18.sp)
                Spacer(modifier = Modifier.height(6.dp))
                Text(
                    text = stringResource(R.string.register_subtitle),
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
                            label = stringResource(R.string.register_username_label),
                            value = username,
                            onValueChange = { username = it },
                            enabled = !isBusy
                        )
                        Spacer(modifier = Modifier.height(12.dp))
                        AuthTextField(
                            label = stringResource(R.string.common_email_label),
                            value = email,
                            onValueChange = { email = it },
                            enabled = !isBusy,
                            keyboardType = KeyboardType.Email
                        )
                        Spacer(modifier = Modifier.height(12.dp))
                        AuthTextField(
                            label = stringResource(R.string.common_password_label),
                            value = password,
                            onValueChange = { password = it },
                            enabled = !isBusy,
                            visualTransformation = PasswordVisualTransformation()
                        )

                        uiState.error?.let { message ->
                            Spacer(modifier = Modifier.height(10.dp))
                            Text(
                                text = message,
                                color = MaterialTheme.colorScheme.error,
                                fontSize = 12.sp
                            )
                        }
                        googleUiState.error?.let { message ->
                            Spacer(modifier = Modifier.height(10.dp))
                            Text(
                                text = message,
                                color = MaterialTheme.colorScheme.error,
                                fontSize = 12.sp
                            )
                        }

                        Spacer(modifier = Modifier.height(14.dp))
                        GradientButton(
                            text = stringResource(R.string.register_submit),
                            onClick = { registerViewModel.register(username, email, password) },
                            enabled = !isBusy
                        ) {
                            if (uiState.isLoading) {
                                CircularProgressIndicator(
                                    modifier = Modifier.size(18.dp),
                                    strokeWidth = 2.dp,
                                    color = MaterialTheme.colorScheme.background
                                )
                            } else {
                                Text(
                                    stringResource(R.string.register_submit),
                                    color = MaterialTheme.colorScheme.background,
                                    fontWeight = FontWeight.SemiBold,
                                    fontSize = 13.sp
                                )
                            }
                        }

                        Spacer(modifier = Modifier.height(14.dp))
                        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.fillMaxWidth()) {
                            ThinDivider(modifier = Modifier.weight(1f))
                            Text(text = "  ${stringResource(R.string.common_divider_or)}  ", fontSize = 11.sp, color = AppFaint)
                            ThinDivider(modifier = Modifier.weight(1f))
                        }
                        Spacer(modifier = Modifier.height(14.dp))

                        GradientOutlineButton(
                            text = stringResource(R.string.common_continue_with_google),
                            onClick = { googleViewModel.loginWithGoogle(context) },
                            enabled = !isBusy
                        )

                        Spacer(modifier = Modifier.height(14.dp))
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.Center
                        ) {
                            Text(text = stringResource(R.string.register_have_account), fontSize = 12.sp, color = AppFaint)
                            Text(
                                text = stringResource(R.string.register_login_link),
                                fontSize = 12.sp,
                                color = AccentSoft,
                                fontWeight = FontWeight.SemiBold,
                                modifier = Modifier.clickable(enabled = !isBusy, onClick = onNavigateToLogin)
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun RegisteredConfirmation(email: String, onNavigateToLogin: () -> Unit) {
    GradientTitle(text = stringResource(R.string.register_check_email_title), fontSize = 18.sp)
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
                text = stringResource(R.string.register_check_email_intro),
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
                text = stringResource(R.string.register_check_email_outro),
                color = AppFaint,
                fontSize = 13.sp,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(18.dp))
            GradientButton(text = stringResource(R.string.register_go_to_login), onClick = onNavigateToLogin)
        }
    }
}
