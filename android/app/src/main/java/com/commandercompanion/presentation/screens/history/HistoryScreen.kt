package com.commandercompanion.presentation.screens.history

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import com.commandercompanion.R
import com.commandercompanion.domain.model.PlayedGame
import com.commandercompanion.presentation.components.AppScreenBackground
import com.commandercompanion.presentation.components.CircleIconButton
import com.commandercompanion.presentation.components.GlassCard
import com.commandercompanion.presentation.components.StatusPill
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppMuted
import com.commandercompanion.presentation.theme.AppOnBackground
import com.commandercompanion.presentation.theme.StatusInfo
import com.commandercompanion.presentation.theme.StatusInfoContainer
import com.commandercompanion.presentation.theme.StatusSuccess
import com.commandercompanion.presentation.theme.StatusSuccessContainer
import com.commandercompanion.presentation.theme.colorForKey
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

@Composable
fun HistoryScreen(
    onBack: () -> Unit,
    viewModel: HistoryViewModel = hiltViewModel()
) {
    val games by viewModel.games.collectAsState()

    // Bottom system inset (the gesture bar) added to the list's own padding, since
    // AppScreenBackground deliberately does not consume it -- see the call below.
    val bottomInset = WindowInsets.safeDrawing.asPaddingValues().calculateBottomPadding()

    // The list underneath scrolls *under* the gesture bar instead of stopping above it, so the
    // bottom inset is left out here and folded into the LazyColumn's contentPadding below.
    AppScreenBackground(
        contentWindowInsets = WindowInsets.safeDrawing
            .only(WindowInsetsSides.Horizontal + WindowInsetsSides.Top)
    ) {
        Column(modifier = Modifier.fillMaxSize()) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(20.dp, 20.dp, 20.dp, 12.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                CircleIconButton(label = "‹", onClick = onBack)
                Text(
                    stringResource(R.string.history_title),
                    color = AppOnBackground,
                    fontWeight = FontWeight.SemiBold,
                    fontSize = 17.sp
                )
            }

            if (games.isEmpty()) {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    Text(
                        stringResource(R.string.history_empty),
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            } else {
                LazyColumn(
                    modifier = Modifier.fillMaxSize().padding(horizontal = 18.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                    contentPadding = PaddingValues(bottom = 20.dp + bottomInset)
                ) {
                    items(games, key = { it.id }) { entry ->
                        GameHistoryCard(entry)
                    }
                }
            }
        }
    }
}

@Composable
private fun GameHistoryCard(entry: PlayedGame) {
    val dateFormat = remember { SimpleDateFormat("dd/MM/yyyy HH:mm", Locale.getDefault()) }
    val winner = entry.seats.firstOrNull { it.won }
    val finished = entry.isFinished

    GlassCard(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(22.dp, 16.dp, 22.dp, 16.dp),
        contentPadding = PaddingValues(horizontal = 18.dp, vertical = 16.dp)
    ) {
        Column(modifier = Modifier.fillMaxWidth()) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = dateFormat.format(Date(entry.startedAtEpochMillis)),
                    color = AppMuted,
                    fontSize = 11.sp
                )
                StatusPill(
                    text = if (finished) stringResource(R.string.history_status_finished) else stringResource(R.string.history_status_in_progress),
                    containerColor = if (finished) StatusSuccessContainer else StatusInfoContainer,
                    contentColor = if (finished) StatusSuccess else StatusInfo
                )
            }
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = winner?.let { stringResource(R.string.history_winner, it.name) }
                    ?: stringResource(R.string.history_player_count, entry.playerCount),
                color = AppOnBackground,
                fontWeight = FontWeight.SemiBold,
                fontSize = 14.sp
            )
            Spacer(modifier = Modifier.height(10.dp))
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                entry.seats.forEach { player ->
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Box(
                            modifier = Modifier
                                .size(9.dp)
                                .clip(CircleShape)
                                .background(colorForKey(player.colorKey))
                        )
                        Spacer(modifier = Modifier.width(5.dp))
                        val label = if (player.mulligans > 0) {
                            stringResource(R.string.history_player_life_with_mulligans, player.name, player.finalLife, player.mulligans)
                        } else {
                            stringResource(R.string.history_player_life, player.name, player.finalLife)
                        }
                        Text(label, color = AppFaint, fontSize = 11.sp)
                    }
                }
            }
        }
    }
}
