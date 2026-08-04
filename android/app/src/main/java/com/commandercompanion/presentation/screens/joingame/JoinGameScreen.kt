package com.commandercompanion.presentation.screens.joingame

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import com.commandercompanion.R
import com.commandercompanion.data.remote.dto.GameDto
import com.commandercompanion.data.remote.dto.PlaygroupDto
import com.commandercompanion.presentation.components.AppScreenBackground
import com.commandercompanion.presentation.components.CircleIconButton
import com.commandercompanion.presentation.components.GlassCard
import com.commandercompanion.presentation.components.GradientButton
import com.commandercompanion.presentation.components.SectionEyebrow
import com.commandercompanion.presentation.components.SelectableChip
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppOnBackground
import com.commandercompanion.presentation.theme.StatusDanger

@Composable
fun JoinGameScreen(
    onBack: () -> Unit,
    onJoined: (gameId: String, localPlayerId: String) -> Unit,
    viewModel: JoinGameViewModel = hiltViewModel()
) {
    val state = viewModel.uiState

    AppScreenBackground {
        Column(modifier = Modifier.fillMaxSize()) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(20.dp, 20.dp, 20.dp, 12.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                CircleIconButton(label = "‹", onClick = onBack)
                Text(
                    stringResource(R.string.join_game_title),
                    color = AppOnBackground,
                    fontWeight = FontWeight.SemiBold,
                    fontSize = 18.sp
                )
            }

            // A single scrolling LazyColumn (header info + game rows + the join button all as
            // items) rather than a fixed header above a weighted games list: in landscape
            // there's much less height to work with, and a non-scrollable header could
            // otherwise push the games list and the join button off-screen with no way to
            // reach them (same issue fixed on PlayerSetupScreen).
            LazyColumn(
                modifier = Modifier.fillMaxSize().padding(horizontal = 20.dp),
                contentPadding = PaddingValues(bottom = 20.dp)
            ) {
                item {
                    Text(
                        stringResource(R.string.join_game_subtitle),
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        fontSize = 12.sp
                    )
                    Spacer(modifier = Modifier.height(16.dp))

                    SectionEyebrow(stringResource(R.string.setup_group_label))
                    Spacer(modifier = Modifier.height(8.dp))
                    if (state.playgroups.isEmpty()) {
                        Text(stringResource(R.string.setup_no_playgroups), color = AppFaint, fontSize = 12.sp)
                    } else {
                        LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            items(state.playgroups) { playgroup ->
                                SelectableChip(
                                    label = playgroup.name,
                                    selected = playgroup.id == state.selectedPlaygroup?.id,
                                    onClick = { viewModel.selectPlaygroup(playgroup) }
                                )
                            }
                        }
                    }

                    if (state.selectedPlaygroup != null) {
                        Spacer(modifier = Modifier.height(20.dp))
                        SectionEyebrow(stringResource(R.string.join_game_open_games_label))
                        Spacer(modifier = Modifier.height(8.dp))
                        when {
                            state.isLoadingGames ->
                                Text(stringResource(R.string.join_game_loading), color = AppFaint, fontSize = 12.sp)
                            state.joinableGames.isEmpty() ->
                                Text(stringResource(R.string.join_game_no_open_games), color = AppFaint, fontSize = 12.sp)
                        }
                    }
                }

                if (state.selectedPlaygroup != null && !state.isLoadingGames && state.joinableGames.isNotEmpty()) {
                    items(state.joinableGames) { game ->
                        GameRow(
                            game = game,
                            playgroup = state.selectedPlaygroup,
                            selected = game.id == state.selectedGameId,
                            onClick = { viewModel.selectGame(game.id) },
                            modifier = Modifier.padding(bottom = 10.dp)
                        )
                    }
                }

                item {
                    if (state.selectedGameId != null) {
                        Spacer(modifier = Modifier.height(16.dp))
                        SectionEyebrow(stringResource(R.string.setup_which_deck))
                        Spacer(modifier = Modifier.height(8.dp))
                        if (state.ownDecks.isEmpty()) {
                            Text(stringResource(R.string.join_game_no_own_decks), color = AppFaint, fontSize = 12.sp)
                        } else {
                            LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                items(state.ownDecks) { deck ->
                                    SelectableChip(
                                        label = deck.name,
                                        selected = deck.id == state.selectedDeckId,
                                        onClick = { viewModel.selectDeck(deck.id) }
                                    )
                                }
                            }
                        }
                    }

                    if (state.error != null) {
                        Spacer(modifier = Modifier.height(10.dp))
                        Text(state.error, color = StatusDanger, fontSize = 12.sp)
                    }

                    Spacer(modifier = Modifier.height(16.dp))
                    GradientButton(
                        text = stringResource(R.string.join_game_join_button),
                        enabled = state.selectedGameId != null && state.selectedDeckId != null && !state.isJoining,
                        onClick = { viewModel.join(onJoined) }
                    )
                }
            }
        }
    }
}

@Composable
private fun GameRow(
    game: GameDto,
    playgroup: PlaygroupDto?,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val usernames = game.players.mapNotNull { player ->
        playgroup?.members?.firstOrNull { it.userId == player.userId }?.username
    }
    GlassCard(modifier = modifier.fillMaxWidth(), shape = RoundedCornerShape(16.dp)) {
        Column(modifier = Modifier.fillMaxWidth()) {
            Text(
                stringResource(R.string.join_game_seats_taken, game.players.size),
                color = AppOnBackground,
                fontWeight = FontWeight.SemiBold,
                fontSize = 13.sp
            )
            if (usernames.isNotEmpty()) {
                Spacer(modifier = Modifier.height(4.dp))
                Text(usernames.joinToString(", "), color = AppFaint, fontSize = 11.sp)
            }
            Spacer(modifier = Modifier.height(8.dp))
            SelectableChip(
                label = stringResource(if (selected) R.string.join_game_selected else R.string.join_game_select),
                selected = selected,
                onClick = onClick
            )
        }
    }
}
