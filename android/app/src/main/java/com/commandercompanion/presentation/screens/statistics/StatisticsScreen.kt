package com.commandercompanion.presentation.screens.statistics

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.SecondaryTabRow
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import com.commandercompanion.R
import com.commandercompanion.data.remote.dto.FinishedGameDto
import com.commandercompanion.data.remote.dto.FinishedGamePlayerDto
import com.commandercompanion.data.remote.dto.OpponentStatsDto
import com.commandercompanion.data.remote.dto.PlaygroupGameCountDto
import com.commandercompanion.data.remote.dto.UserStatsDto
import com.commandercompanion.domain.model.DeckWithStats
import com.commandercompanion.presentation.components.AppScreenBackground
import com.commandercompanion.presentation.components.CircleIconButton
import com.commandercompanion.presentation.components.DeckThumbnail
import com.commandercompanion.presentation.components.GlassCard
import com.commandercompanion.presentation.components.SectionEyebrow
import com.commandercompanion.presentation.components.SelectableChip
import com.commandercompanion.presentation.components.StatusPill
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppMuted
import com.commandercompanion.presentation.theme.AppOnBackground
import com.commandercompanion.presentation.theme.StatusDanger
import com.commandercompanion.presentation.theme.StatusSuccess
import com.commandercompanion.presentation.theme.StatusSuccessContainer
import java.time.OffsetDateTime
import java.time.format.DateTimeFormatter
import java.time.format.DateTimeParseException
import java.util.Locale

@Composable
fun StatisticsScreen(
    onBack: () -> Unit,
    viewModel: StatisticsViewModel = hiltViewModel(),
    finishedGamesViewModel: FinishedGamesViewModel = hiltViewModel()
) {
    val state by viewModel.uiState.collectAsState()
    val finishedGamesState by finishedGamesViewModel.uiState.collectAsState()
    var selectedTab by rememberSaveable { mutableIntStateOf(0) }

    AppScreenBackground {
        Column(modifier = Modifier.fillMaxSize().padding(20.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                CircleIconButton(label = "‹", onClick = onBack)
                Text(stringResource(R.string.statistics_title), color = AppOnBackground, fontSize = 17.sp)
            }
            Spacer(modifier = Modifier.height(16.dp))

            when {
                state.isLoading -> Column(
                    modifier = Modifier.fillMaxSize(),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center
                ) { CircularProgressIndicator() }

                state.loadError -> Text(
                    stringResource(R.string.statistics_load_error),
                    color = StatusDanger,
                    fontSize = 13.sp
                )

                else -> Column(
                    modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()),
                    verticalArrangement = Arrangement.spacedBy(20.dp)
                ) {
                    state.userStats?.let { GlobalStatsSection(it) }
                    HeadToHeadSection(mostPlayed = state.mostPlayedOpponent, archenemy = state.archenemy)
                    MostPlayedGroupSection(state.mostPlayedPlaygroup)

                    SecondaryTabRow(selectedTabIndex = selectedTab) {
                        Tab(
                            selected = selectedTab == 0,
                            onClick = { selectedTab = 0 },
                            text = { Text(stringResource(R.string.statistics_tab_by_deck)) }
                        )
                        Tab(
                            selected = selectedTab == 1,
                            onClick = { selectedTab = 1 },
                            text = { Text(stringResource(R.string.statistics_tab_finished_games)) }
                        )
                    }

                    when (selectedTab) {
                        0 -> DeckStatsSection(
                            deckStats = state.sortedDeckStats,
                            sortOrder = state.deckSortOrder,
                            onSortOrderChange = viewModel::setDeckSortOrder
                        )
                        else -> FinishedGamesSection(
                            state = finishedGamesState,
                            onLoadMore = finishedGamesViewModel::loadMore
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun GlobalStatsSection(stats: UserStatsDto) {
    Column {
        SectionEyebrow(text = stringResource(R.string.statistics_global_heading))
        Spacer(modifier = Modifier.height(10.dp))

        val winRate = winRatePercent(stats.gamesPlayed, stats.gamesWon)
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            StatTile(stringResource(R.string.statistics_games), stats.gamesPlayed.toString(), Modifier.weight(1f))
            StatTile(stringResource(R.string.statistics_wins), stats.gamesWon.toString(), Modifier.weight(1f))
            StatTile(stringResource(R.string.statistics_win_rate), "$winRate%", Modifier.weight(1f))
        }
        Spacer(modifier = Modifier.height(10.dp))
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            StatTile(stringResource(R.string.statistics_total_damage), stats.totalDamageDealt.toString(), Modifier.weight(1f))
            StatTile(stringResource(R.string.statistics_commander_damage), stats.totalCommanderDamageDealt.toString(), Modifier.weight(1f))
            StatTile(stringResource(R.string.statistics_eliminations), stats.totalEliminations.toString(), Modifier.weight(1f))
        }
    }
}

/** "Who you play the most" / "Archenemy" cards, derived from [OpponentStatsDto]. */
@Composable
private fun HeadToHeadSection(mostPlayed: OpponentStatsDto?, archenemy: OpponentStatsDto?) {
    Column {
        SectionEyebrow(text = stringResource(R.string.statistics_head_to_head_heading))
        Spacer(modifier = Modifier.height(10.dp))

        if (mostPlayed == null && archenemy == null) {
            Text(stringResource(R.string.statistics_no_head_to_head), color = AppMuted, fontSize = 13.sp)
            return@Column
        }

        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            mostPlayed?.let {
                HeadToHeadCard(
                    label = stringResource(R.string.statistics_most_played_opponent_label),
                    username = it.username,
                    detail = stringResource(R.string.statistics_opponent_games_together, it.gamesTogether),
                    modifier = Modifier.weight(1f)
                )
            }
            archenemy?.let {
                HeadToHeadCard(
                    label = stringResource(R.string.statistics_archenemy_label),
                    username = it.username,
                    detail = stringResource(R.string.statistics_archenemy_summary, it.timesEliminatedByOpponent),
                    modifier = Modifier.weight(1f)
                )
            }
        }
    }
}

@Composable
private fun HeadToHeadCard(label: String, username: String, detail: String, modifier: Modifier = Modifier) {
    GlassCard(modifier = modifier, contentPadding = PaddingValues(horizontal = 14.dp, vertical = 14.dp)) {
        Column {
            Text(label, color = AppFaint, fontSize = 11.sp)
            Spacer(modifier = Modifier.height(4.dp))
            Text(username, color = AppOnBackground, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
            Spacer(modifier = Modifier.height(2.dp))
            Text(detail, color = AppMuted, fontSize = 12.sp)
        }
    }
}

@Composable
private fun MostPlayedGroupSection(group: PlaygroupGameCountDto?) {
    Column {
        SectionEyebrow(text = stringResource(R.string.statistics_most_played_group_label))
        Spacer(modifier = Modifier.height(10.dp))

        if (group == null) {
            Text(stringResource(R.string.statistics_no_group_data), color = AppMuted, fontSize = 13.sp)
            return@Column
        }

        GlassCard(modifier = Modifier.fillMaxWidth()) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(group.playgroupName, color = AppOnBackground, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
                Text(
                    stringResource(R.string.statistics_group_games_played, group.gamesPlayed),
                    color = AppFaint,
                    fontSize = 12.sp
                )
            }
        }
    }
}

@Composable
private fun DeckStatsSection(
    deckStats: List<DeckWithStats>,
    sortOrder: DeckSortOrder,
    onSortOrderChange: (DeckSortOrder) -> Unit
) {
    Column {
        if (deckStats.isEmpty()) {
            Text(stringResource(R.string.statistics_no_decks), color = AppMuted, fontSize = 13.sp)
            return@Column
        }

        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            SelectableChip(
                label = stringResource(R.string.statistics_sort_recent),
                selected = sortOrder == DeckSortOrder.RECENT,
                onClick = { onSortOrderChange(DeckSortOrder.RECENT) }
            )
            SelectableChip(
                label = stringResource(R.string.statistics_sort_win_rate),
                selected = sortOrder == DeckSortOrder.WIN_RATE,
                onClick = { onSortOrderChange(DeckSortOrder.WIN_RATE) }
            )
            SelectableChip(
                label = stringResource(R.string.statistics_sort_games_played),
                selected = sortOrder == DeckSortOrder.GAMES_PLAYED,
                onClick = { onSortOrderChange(DeckSortOrder.GAMES_PLAYED) }
            )
        }
        Spacer(modifier = Modifier.height(14.dp))

        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            deckStats.forEach { entry -> DeckStatsCard(entry) }
        }
    }
}

@Composable
private fun DeckStatsCard(entry: DeckWithStats) {
    GlassCard(modifier = Modifier.fillMaxWidth()) {
        Row(modifier = Modifier.fillMaxWidth()) {
            DeckThumbnail(commander = entry.deck.commander, imageUrl = entry.deck.imageUrl, size = 56.dp)
            Spacer(modifier = Modifier.width(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(entry.deck.name, color = AppOnBackground, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
                    Text(entry.deck.commander, color = AppFaint, fontSize = 12.sp)
                }

                val stats = entry.stats
                if (stats == null) {
                    Spacer(modifier = Modifier.height(6.dp))
                    Text(stringResource(R.string.statistics_no_deck_stats), color = AppMuted, fontSize = 12.sp)
                } else {
                    Spacer(modifier = Modifier.height(10.dp))
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        DeckStatValue(stringResource(R.string.statistics_games), stats.gamesPlayed.toString(), Modifier.weight(1f))
                        DeckStatValue(stringResource(R.string.statistics_wins), stats.gamesWon.toString(), Modifier.weight(1f))
                        DeckStatValue(stringResource(R.string.statistics_deck_max_life), stats.highestLifeTotalAchieved.toString(), Modifier.weight(1f))
                        DeckStatValue(stringResource(R.string.statistics_commander_damage), stats.totalCommanderDamageDealt.toString(), Modifier.weight(1f))
                    }
                }
            }
        }
    }
}

@Composable
private fun FinishedGamesSection(state: FinishedGamesUiState, onLoadMore: () -> Unit) {
    Column {
        when {
            state.isLoading -> Column(
                modifier = Modifier.fillMaxWidth(),
                horizontalAlignment = Alignment.CenterHorizontally
            ) { CircularProgressIndicator() }

            state.loadError -> Text(stringResource(R.string.statistics_load_error), color = StatusDanger, fontSize = 13.sp)

            state.games.isEmpty() -> Text(stringResource(R.string.statistics_no_finished_games), color = AppMuted, fontSize = 13.sp)

            else -> {
                Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    state.games.forEach { game -> FinishedGameCard(game) }
                }
                if (state.hasMore) {
                    Spacer(modifier = Modifier.height(14.dp))
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
                        SelectableChip(
                            label = stringResource(R.string.statistics_load_more),
                            selected = false,
                            onClick = onLoadMore
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun FinishedGameCard(game: FinishedGameDto) {
    GlassCard(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.fillMaxWidth()) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    game.playgroupName ?: stringResource(R.string.statistics_game_no_playgroup),
                    color = AppOnBackground,
                    fontWeight = FontWeight.SemiBold,
                    fontSize = 13.sp
                )
                Text(formatGameDate(game.finishedAt), color = AppFaint, fontSize = 11.sp)
            }
            Spacer(modifier = Modifier.height(10.dp))
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                game.players.forEach { player -> FinishedGamePlayerRow(player) }
            }
        }
    }
}

@Composable
private fun FinishedGamePlayerRow(player: FinishedGamePlayerDto) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            DeckThumbnail(commander = player.deckCommander, imageUrl = player.deckImageUrl, size = 32.dp)
            Column {
                Text(player.username, color = AppOnBackground, fontSize = 13.sp)
                Text(player.deckName, color = AppFaint, fontSize = 11.sp)
            }
        }
        StatusPill(
            text = if (player.won) {
                stringResource(R.string.statistics_game_result_won)
            } else {
                stringResource(R.string.statistics_game_result_lost)
            },
            containerColor = if (player.won) StatusSuccessContainer else StatusDanger.copy(alpha = 0.15f),
            contentColor = if (player.won) StatusSuccess else StatusDanger
        )
    }
}

@Composable
private fun StatTile(label: String, value: String, modifier: Modifier = Modifier) {
    GlassCard(modifier = modifier, contentPadding = PaddingValues(horizontal = 12.dp, vertical = 14.dp)) {
        Column {
            Text(value, color = AppOnBackground, fontWeight = FontWeight.Bold, fontSize = 18.sp)
            Spacer(modifier = Modifier.height(2.dp))
            Text(label, color = AppFaint, fontSize = 11.sp)
        }
    }
}

@Composable
private fun DeckStatValue(label: String, value: String, modifier: Modifier = Modifier) {
    Column(modifier = modifier) {
        Text(value, color = AppOnBackground, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
        Text(label, color = AppFaint, fontSize = 10.sp)
    }
}

private val gameDateFormatter: DateTimeFormatter = DateTimeFormatter.ofPattern("dd/MM/yyyy HH:mm", Locale.getDefault())

/** Best-effort formatting: falls back to the raw ISO string on a parse failure rather than crashing the row. */
private fun formatGameDate(isoString: String?): String {
    if (isoString == null) return ""
    return try {
        OffsetDateTime.parse(isoString).format(gameDateFormatter)
    } catch (e: DateTimeParseException) {
        isoString
    }
}
