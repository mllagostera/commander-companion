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

/** Se cancela el picker de cuentas de Google (el usuario cerró el bottom sheet). */
class GoogleSignInCancelledException : Exception("El usuario canceló el picker de cuentas de Google")

/** No hay ninguna cuenta de Google configurada en el dispositivo. */
class NoGoogleAccountException : Exception("No hay ninguna cuenta de Google configurada en este dispositivo")

/**
 * Envoltorio sobre Credential Manager + Google Identity Services para obtener el `id_token`
 * que se manda a `POST /auth/google`.
 *
 * IMPORTANTE: `BuildConfig.GOOGLE_WEB_CLIENT_ID` es un placeholder hasta que se creen las
 * credenciales OAuth reales en Google Cloud Console (ver `docs/roadmap/TASKS.md` Stage 1 y el
 * comentario en `app/build.gradle.kts`). Con el placeholder, `getIdToken` va a fallar de forma
 * predecible (Google rechaza el client id) — el código del flujo es correcto y completo, pero no
 * se puede probar end-to-end contra Google real hasta ese paso manual.
 */
@Singleton
class GoogleAuthClient @Inject constructor() {

    /**
     * Dispara el picker de cuentas de Google y devuelve el `id_token` obtenido.
     * Requiere un [context] de Activity (Credential Manager necesita poder mostrar UI).
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
     * Limpia el estado de credenciales de Google guardado por Credential Manager (auto-select,
     * etc.) al cerrar sesión. Best-effort: si falla no debe bloquear el logout.
     */
    suspend fun clearCredentialState(context: Context) {
        try {
            CredentialManager.create(context).clearCredentialState(ClearCredentialStateRequest())
        } catch (e: ClearCredentialException) {
            // Best-effort: no hay nada razonable que hacer si esto falla, no debe romper el logout.
        }
    }
}
