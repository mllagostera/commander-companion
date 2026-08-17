package com.commandercompanion.presentation.screens.friends

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.runtime.collectAsState
import androidx.hilt.navigation.compose.hiltViewModel
import com.commandercompanion.R
import com.commandercompanion.presentation.components.AppScreenBackground
import com.commandercompanion.presentation.components.AuthTextField
import com.commandercompanion.presentation.components.CircleIconButton
import com.commandercompanion.presentation.components.GlassCard
import com.commandercompanion.presentation.components.GradientOutlineButton
import com.commandercompanion.presentation.components.SectionEyebrow
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppOnBackground
import com.commandercompanion.presentation.theme.AppOnSurfaceVariant
import com.commandercompanion.presentation.theme.StatusDanger
import com.commandercompanion.presentation.theme.StatusSuccess

/** Maps the ViewModel's error enum onto the translated strings (see [FriendsError]). */
@Composable
private fun FriendsError.message(): String = stringResource(
    when (this) {
        FriendsError.NETWORK -> R.string.error_friends_network
        FriendsError.SELF -> R.string.error_friends_self
        FriendsError.USER_NOT_FOUND -> R.string.error_friends_user_not_found
        FriendsError.ALREADY_RELATED -> R.string.error_friends_already_related
        FriendsError.REQUEST_GONE -> R.string.error_friends_request_gone
        FriendsError.UNKNOWN -> R.string.error_friends_unknown
    }
)

@Composable
fun FriendsScreen(
    onBack: () -> Unit,
    viewModel: FriendsViewModel = hiltViewModel()
) {
    val state by viewModel.uiState.collectAsState()

    AppScreenBackground {
        Column(modifier = Modifier.fillMaxSize()) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(20.dp, 20.dp, 20.dp, 12.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                CircleIconButton(label = "‹", onClick = onBack)
                Text(
                    stringResource(R.string.friends_title),
                    color = AppOnBackground,
                    fontWeight = FontWeight.SemiBold,
                    fontSize = 18.sp
                )
            }

            // One scrolling list for everything, same reasoning as JoinGameScreen:
            // a fixed header above a weighted list pushes content off-screen in
            // landscape with no way to reach it.
            LazyColumn(
                modifier = Modifier.fillMaxSize().padding(horizontal = 20.dp),
                contentPadding = PaddingValues(bottom = 20.dp)
            ) {
                item {
                    Text(
                        stringResource(R.string.friends_subtitle),
                        color = AppOnSurfaceVariant,
                        fontSize = 12.sp
                    )
                    Spacer(modifier = Modifier.height(16.dp))

                    AuthTextField(
                        label = stringResource(R.string.friends_search_label),
                        value = state.query,
                        onValueChange = viewModel::onQueryChange,
                        enabled = true
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                }

                if (state.isSearching) {
                    item { Hint(stringResource(R.string.friends_searching)) }
                }

                if (state.query.isNotBlank() && !state.isSearching && state.results.isEmpty()) {
                    item { Hint(stringResource(R.string.friends_no_results)) }
                }

                items(state.results, key = { "result-${it.id}" }) { result ->
                    val known = result.id in state.knownUserIds
                    PersonRow(
                        username = result.username,
                        busy = result.id in state.busyIds,
                        // Offering "add" for someone already on the list would
                        // just earn a 409, so it says so instead.
                        actionLabel = if (known) null else stringResource(R.string.friends_action_add),
                        onAction = { viewModel.sendRequest(result.id) },
                        note = if (known) stringResource(R.string.friends_already_related) else null
                    )
                }

                state.lastOutcome?.let { outcome ->
                    item {
                        Hint(
                            text = stringResource(
                                when (outcome) {
                                    SendOutcome.REQUEST_SENT -> R.string.friends_outcome_sent
                                    SendOutcome.FRIENDS_NOW -> R.string.friends_outcome_now_friends
                                }
                            ),
                            color = StatusSuccess
                        )
                    }
                }

                state.actionError?.let { error ->
                    item { Hint(text = error.message(), color = StatusDanger) }
                }

                state.loadError?.let { error ->
                    item {
                        Spacer(modifier = Modifier.height(12.dp))
                        Hint(text = error.message(), color = StatusDanger)
                        Spacer(modifier = Modifier.height(8.dp))
                        GradientOutlineButton(
                            text = stringResource(R.string.friends_retry),
                            onClick = viewModel::load
                        )
                    }
                }

                if (!state.isLoading && state.loadError == null && state.isEmpty && state.results.isEmpty()) {
                    item {
                        Spacer(modifier = Modifier.height(12.dp))
                        Hint(stringResource(R.string.friends_empty))
                    }
                }

                if (state.incoming.isNotEmpty()) {
                    item { Section(stringResource(R.string.friends_section_incoming)) }
                    items(state.incoming, key = { "in-${it.id}" }) { request ->
                        PersonRow(
                            username = request.requesterUsername,
                            busy = request.id in state.busyIds,
                            actionLabel = stringResource(R.string.friends_action_accept),
                            onAction = { viewModel.acceptRequest(request.id) },
                            secondaryLabel = stringResource(R.string.friends_action_reject),
                            onSecondary = { viewModel.rejectRequest(request.id) }
                        )
                    }
                }

                if (state.outgoing.isNotEmpty()) {
                    item { Section(stringResource(R.string.friends_section_outgoing)) }
                    items(state.outgoing, key = { "out-${it.id}" }) { request ->
                        PersonRow(
                            username = request.addresseeUsername,
                            busy = request.id in state.busyIds,
                            actionLabel = stringResource(R.string.friends_action_cancel),
                            onAction = { viewModel.cancelRequest(request.id) }
                        )
                    }
                }

                if (state.friends.isNotEmpty()) {
                    item { Section(stringResource(R.string.friends_section_list)) }
                    items(state.friends, key = { "friend-${it.id}" }) { friend ->
                        PersonRow(
                            username = friend.username,
                            busy = friend.id in state.busyIds,
                            actionLabel = stringResource(R.string.friends_action_remove),
                            onAction = { viewModel.removeFriend(friend.id) }
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun Section(title: String) {
    Spacer(modifier = Modifier.height(20.dp))
    SectionEyebrow(title)
    Spacer(modifier = Modifier.height(8.dp))
}

@Composable
private fun Hint(text: String, color: androidx.compose.ui.graphics.Color = AppFaint) {
    Text(text = text, color = color, fontSize = 12.sp)
}

/**
 * One person plus the actions available on them. The same row serves search
 * results, both request directions and the friends list — they differ only in
 * which buttons they carry.
 */
@Composable
private fun PersonRow(
    username: String,
    busy: Boolean,
    actionLabel: String?,
    onAction: () -> Unit,
    secondaryLabel: String? = null,
    onSecondary: (() -> Unit)? = null,
    note: String? = null
) {
    GlassCard(modifier = Modifier.fillMaxWidth().padding(bottom = 8.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(username, color = AppOnBackground, fontSize = 14.sp, fontWeight = FontWeight.Medium)
                note?.let { Text(it, color = AppFaint, fontSize = 11.sp) }
            }
            if (secondaryLabel != null && onSecondary != null) {
                GradientOutlineButton(
                    text = secondaryLabel,
                    onClick = onSecondary,
                    enabled = !busy,
                    modifier = Modifier.weight(1f)
                )
            }
            if (actionLabel != null) {
                GradientOutlineButton(
                    text = actionLabel,
                    onClick = onAction,
                    enabled = !busy,
                    modifier = Modifier.weight(1f)
                )
            }
        }
    }
}
