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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import com.commandercompanion.R
import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.UserStatsDto
import com.commandercompanion.domain.model.DeckWithStats
import com.commandercompanion.domain.model.PlaygroupSummary
import com.commandercompanion.presentation.components.AppScreenBackground
import com.commandercompanion.presentation.components.CircleIconButton
import com.commandercompanion.presentation.components.GlassCard
import com.commandercompanion.presentation.components.SectionEyebrow
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppMuted
import com.commandercompanion.presentation.theme.AppOnBackground
import com.commandercompanion.presentation.theme.StatusDanger
import kotlin.math.roundToInt

@Composable
fun StatisticsScreen(
    onBack: () -> Unit,
    viewModel: StatisticsViewModel = hiltViewModel()
) {
    val state by viewModel.uiState.collectAsState()

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
                    DeckStatsSection(state.deckStats)
                    PlaygroupStatsSection(state.playgroupSummaries)
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

@Composable
private fun DeckStatsSection(deckStats: List<DeckWithStats>) {
    Column {
        SectionEyebrow(text = stringResource(R.string.statistics_per_deck_heading))
        Spacer(modifier = Modifier.height(10.dp))

        if (deckStats.isEmpty()) {
            Text(stringResource(R.string.statistics_no_decks), color = AppMuted, fontSize = 13.sp)
            return@Column
        }

        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            deckStats.forEach { entry -> DeckStatsCard(entry) }
        }
    }
}

@Composable
private fun DeckStatsCard(entry: DeckWithStats) {
    GlassCard(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.fillMaxWidth()) {
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

@Composable
private fun PlaygroupStatsSection(summaries: List<PlaygroupSummary>) {
    Column {
        SectionEyebrow(text = stringResource(R.string.statistics_per_group_heading))
        Spacer(modifier = Modifier.height(10.dp))

        if (summaries.isEmpty()) {
            Text(stringResource(R.string.statistics_no_groups), color = AppMuted, fontSize = 13.sp)
            return@Column
        }

        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            summaries.forEach { summary ->
                GlassCard(modifier = Modifier.fillMaxWidth()) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(summary.playgroup.name, color = AppOnBackground, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
                        Text(
                            stringResource(
                                R.string.statistics_group_summary,
                                summary.gamesPlayed,
                                summary.playgroup.members.size
                            ),
                            color = AppFaint,
                            fontSize = 12.sp
                        )
                    }
                }
            }
        }
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

private fun winRatePercent(played: Int, won: Int): Int =
    if (played == 0) 0 else ((won.toDouble() / played) * 100).roundToInt()
