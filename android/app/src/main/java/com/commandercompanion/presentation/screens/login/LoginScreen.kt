package com.commandercompanion.presentation.screens.login

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
import com.commandercompanion.presentation.theme.AccentSoft
import com.commandercompanion.presentation.theme.AppFaint

/**
 * Real login: email/password against `POST /auth/login`, Google Sign-In (Credential Manager)
 * against `POST /auth/google`, both via [LoginViewModel]. This used to be a pure navigation
 * shell that didn't authenticate against anything (see the decision history in `docs/roadmap/TASKS.md`).
 */
@Composable
fun LoginScreen(
    onLoginSuccess: () -> Unit,
    onNavigateToRegister: () -> Unit,
    viewModel: LoginViewModel = hiltViewModel()
) {
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    val uiState by viewModel.uiState.collectAsState()
    val context = LocalContext.current

    LaunchedEffect(uiState.loginSucceeded) {
        if (uiState.loginSucceeded) onLoginSuccess()
    }

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
            GradientTitle(text = stringResource(R.string.app_name), fontSize = 20.sp)
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = stringResource(R.string.login_subtitle),
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                fontSize = 13.sp
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
                    Spacer(modifier = Modifier.height(12.dp))
                    AuthTextField(
                        label = stringResource(R.string.common_password_label),
                        value = password,
                        onValueChange = { password = it },
                        enabled = !uiState.isLoading,
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

                    Spacer(modifier = Modifier.height(14.dp))
                    GradientButton(
                        text = stringResource(R.string.login_submit),
                        onClick = { viewModel.loginWithPassword(email, password) },
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
                                stringResource(R.string.login_submit),
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
                        onClick = { viewModel.loginWithGoogle(context) },
                        enabled = !uiState.isLoading
                    )

                    Spacer(modifier = Modifier.height(14.dp))
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.Center
                    ) {
                        Text(text = stringResource(R.string.login_no_account), fontSize = 12.sp, color = AppFaint)
                        Text(
                            text = stringResource(R.string.login_register_link),
                            fontSize = 12.sp,
                            color = AccentSoft,
                            fontWeight = FontWeight.SemiBold,
                            modifier = Modifier.clickable(
                                enabled = !uiState.isLoading,
                                onClick = onNavigateToRegister
                            )
                        )
                    }
                }
            }
        }
    }
}
