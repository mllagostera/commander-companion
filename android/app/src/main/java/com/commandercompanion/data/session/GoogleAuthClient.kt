package com.commandercompanion.data.session

import android.content.Context
import androidx.credentials.ClearCredentialStateRequest
import androidx.credentials.CredentialManager
import androidx.credentials.CustomCredential
import androidx.credentials.GetCredentialRequest
import androidx.credentials.exceptions.ClearCredentialException
import androidx.credentials.exceptions.GetCredentialCancellationException
import androidx.credentials.exceptions.GetCredentialException
import androidx.credentials.exceptions.NoCredentialException
import com.commandercompanion.BuildConfig
import com.google.android.libraries.identity.googleid.GetGoogleIdOption
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential
import com.google.android.libraries.identity.googleid.GoogleIdTokenParsingException
import javax.inject.Inject
import javax.inject.Singleton

/** The Google account picker was cancelled (the user closed the bottom sheet). */
class GoogleSignInCancelledException : Exception("El usuario canceló el picker de cuentas de Google")

/** There is no Google account configured on the device. */
class NoGoogleAccountException : Exception("No hay ninguna cuenta de Google configurada en este dispositivo")

/**
 * Wrapper over Credential Manager + Google Identity Services to obtain the `id_token`
 * that gets sent to `POST /auth/google`.
 *
 * IMPORTANT: `BuildConfig.GOOGLE_WEB_CLIENT_ID` is a placeholder until real OAuth credentials
 * are created in Google Cloud Console (see `docs/roadmap/TASKS.md` Stage 1 and the comment
 * in `app/build.gradle.kts`). With the placeholder, `getIdToken` will fail in a predictable
 * way (Google rejects the client id) — the flow's code is correct and complete, but it can't
 * be tested end-to-end against real Google until that manual step happens.
 */
@Singleton
class GoogleAuthClient @Inject constructor() {

    /**
     * Triggers the Google account picker and returns the obtained `id_token`.
     * Requires an Activity [context] (Credential Manager needs to be able to show UI).
     */
    suspend fun getIdToken(context: Context): Result<String> {
        val googleIdOption = GetGoogleIdOption.Builder()
            .setFilterByAuthorizedAccounts(false)
            .setServerClientId(BuildConfig.GOOGLE_WEB_CLIENT_ID)
            .setAutoSelectEnabled(false)
            .build()

        val request = GetCredentialRequest.Builder()
            .addCredentialOption(googleIdOption)
            .build()

        return try {
            val credentialManager = CredentialManager.create(context)
            val response = credentialManager.getCredential(context, request)
            val credential = response.credential
            if (credential is CustomCredential &&
                credential.type == GoogleIdTokenCredential.TYPE_GOOGLE_ID_TOKEN_CREDENTIAL
            ) {
                val googleIdTokenCredential = GoogleIdTokenCredential.createFrom(credential.data)
                Result.success(googleIdTokenCredential.idToken)
            } else {
                Result.failure(IllegalStateException("Credential inesperada de Credential Manager"))
            }
        } catch (e: GetCredentialCancellationException) {
            Result.failure(GoogleSignInCancelledException())
        } catch (e: NoCredentialException) {
            Result.failure(NoGoogleAccountException())
        } catch (e: GetCredentialException) {
            Result.failure(e)
        } catch (e: GoogleIdTokenParsingException) {
            Result.failure(e)
        }
    }

    /**
     * Clears the Google credential state saved by Credential Manager (auto-select,
     * etc.) on logout. Best-effort: a failure here must not block the logout.
     */
    suspend fun clearCredentialState(context: Context) {
        try {
            CredentialManager.create(context).clearCredentialState(ClearCredentialStateRequest())
        } catch (e: ClearCredentialException) {
            // Best-effort: there's nothing reasonable to do if this fails, it must not break logout.
        }
    }
}
