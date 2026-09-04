package com.commandercompanion.presentation.components

import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import com.commandercompanion.R
import com.commandercompanion.core.util.ApiFailure

/**
 * Turns the ViewModels' [ApiFailure] into the translated string. Lives here rather than next to
 * [ApiFailure] itself so `core/util` stays free of Compose and of `R`, and so the screens that
 * only ever surface a generic API error don't each repeat this `when`.
 */
@Composable
fun ApiFailure.message(): String = when (this) {
    ApiFailure.Network -> stringResource(R.string.error_api_network)
    ApiFailure.SessionExpired -> stringResource(R.string.error_api_session_expired)
    ApiFailure.Forbidden -> stringResource(R.string.error_api_forbidden)
    ApiFailure.NotFound -> stringResource(R.string.error_api_not_found)
    ApiFailure.Conflict -> stringResource(R.string.error_api_conflict)
    is ApiFailure.Server -> stringResource(R.string.error_api_server, code)
    ApiFailure.Unexpected -> stringResource(R.string.error_api_unexpected)
}
