package com.commandercompanion.presentation.screens.game

import android.content.res.Configuration
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import com.commandercompanion.R
import com.commandercompanion.presentation.components.GradientButton
import com.commandercompanion.presentation.components.RotateDevicePrompt
import com.commandercompanion.presentation.theme.AccentSoft
import com.commandercompanion.presentation.theme.AppBackgroundDeep
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppMuted
import com.commandercompanion.presentation.theme.AppOnBackground
import com.commandercompanion.presentation.theme.AppOutline
import com.commandercompanion.presentation.theme.StatusDanger
import com.commandercompanion.presentation.theme.StatusPoison
import kotlinx.coroutines.delay

@Composable
fun GameTrackerScreen(
    onFinish: () -> Unit,
    viewModel: GameViewModel = hiltViewModel()
) {
    val state by viewModel.state
    var paused by rememberSaveable { mutableStateOf(false) }
    var expandedPlayerId by rememberSaveable { mutableStateOf<Int?>(null) }
    var showStarterBanner by rememberSaveable { mutableStateOf(state.startingPlayerId != null) }

    LaunchedEffect(Unit) {
        if (showStarterBanner) {
            delay(1800)
            showStarterBanner = false
        }
    }

    val isLandscape = LocalConfiguration.current.orientation == Configuration.ORIENTATION_LANDSCAPE

    Box(modifier = Modifier.fillMaxSize().background(AppBackgroundDeep)) {
        when {
            !isLandscape -> RotateDevicePrompt(message = stringResource(R.string.tracker_rotate_prompt))
            state.isFinished -> GameSummary(state = state, onBack = onFinish)
            else -> {
                QuadrantGrid(
                    players = state.players,
                    expandedPlayerId = expandedPlayerId,
                    onToggleExpand = { id -> expandedPlayerId = if (expandedPlayerId == id) null else id },
                    onLifeChange = viewModel::adjustLife,
                    onCommanderDamageChange = viewModel::adjustCommanderDamage,
                    onPoisonChange = viewModel::adjustPoison,
                    onPassTurn = { viewModel.nextTurn() }
                )

                Text(
                    text = state.currentTurn.toString(),
                    color = Color.White.copy(alpha = 0.05f),
                    fontWeight = FontWeight.ExtraBold,
                    fontSize = 96.sp,
                    modifier = Modifier.align(Alignment.Center)
                )

                RemoteSyncBanner(
                    remoteSync = state.remoteSync,
                    modifier = Modifier.align(Alignment.TopCenter).padding(top = 4.dp)
                )

                PauseButton(
                    onClick = { paused = !paused },
                    modifier = Modifier.align(Alignment.Center)
                )

                if (showStarterBanner && state.startingPlayerId != null) {
                    val starterName = state.players.firstOrNull { it.id == state.startingPlayerId }?.name
                    StarterBanner(name = starterName ?: "", modifier = Modifier.matchParentSize())
                }

                if (paused) {
                    PauseOverlay(
                        onResume = { paused = false },
                        onEnd = { viewModel.finishGame() },
                        modifier = Modifier.matchParentSize()
                    )
                }
            }
        }
    }
}

/** Seat grid: first half "at the top of the table" (rotated 180°), the rest below. Works for 2-6. */
@Composable
private fun QuadrantGrid(
    players: List<PlayerState>,
    expandedPlayerId: Int?,
    onToggleExpand: (Int) -> Unit,
    onLifeChange: (playerId: Int, amount: Int) -> Unit,
    onCommanderDamageChange: (targetPlayerId: Int, attackerId: Int, amount: Int) -> Unit,
    onPoisonChange: (playerId: Int, amount: Int) -> Unit,
    onPassTurn: () -> Unit
) {
    val topCount = (players.size + 1) / 2
    Column(
        modifier = Modifier.fillMaxSize().padding(4.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp)
    ) {
        Row(modifier = Modifier.weight(1f).fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            players.take(topCount).forEach { player ->
                PlayerQuadrant(
                    player = player,
                    opponents = players.filter { it.id != player.id },
                    expanded = expandedPlayerId == player.id,
                    onToggleExpand = { onToggleExpand(player.id) },
                    onLifeChange = { delta -> onLifeChange(player.id, delta) },
                    onCommanderDamageChange = { attackerId, delta -> onCommanderDamageChange(player.id, attackerId, delta) },
                    onPoisonChange = { delta -> onPoisonChange(player.id, delta) },
                    onPassTurn = onPassTurn,
                    rotated = true,
                    modifier = Modifier.weight(1f).fillMaxHeight()
                )
            }
        }
        Row(modifier = Modifier.weight(1f).fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            players.drop(topCount).forEach { player ->
                PlayerQuadrant(
                    player = player,
                    opponents = players.filter { it.id != player.id },
                    expanded = expandedPlayerId == player.id,
                    onToggleExpand = { onToggleExpand(player.id) },
                    onLifeChange = { delta -> onLifeChange(player.id, delta) },
                    onCommanderDamageChange = { attackerId, delta -> onCommanderDamageChange(player.id, attackerId, delta) },
                    onPoisonChange = { delta -> onPoisonChange(player.id, delta) },
                    onPassTurn = onPassTurn,
                    rotated = false,
                    modifier = Modifier.weight(1f).fillMaxHeight()
                )
            }
        }
    }
}

@Composable
private fun PlayerQuadrant(
    player: PlayerState,
    opponents: List<PlayerState>,
    expanded: Boolean,
    onToggleExpand: () -> Unit,
    onLifeChange: (Int) -> Unit,
    onCommanderDamageChange: (attackerId: Int, delta: Int) -> Unit,
    onPoisonChange: (Int) -> Unit,
    onPassTurn: () -> Unit,
    rotated: Boolean,
    modifier: Modifier = Modifier
) {
    val eliminated = player.isEliminated()
    val deathAlpha by animateFloatAsState(targetValue = if (eliminated) 1f else 0f, animationSpec = tween(900), label = "death")

    Box(
        modifier = modifier
            .then(if (rotated) Modifier.rotate(180f) else Modifier)
            .clip(RoundedCornerShape(22.dp))
            .background(player.color)
            .clickable(onClick = onToggleExpand)
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(Brush.verticalGradient(listOf(Color(0x0D0A0714), Color(0x730A0714))))
        )

        Column(
            modifier = Modifier.fillMaxSize().padding(10.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.SpaceBetween
        ) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Text(player.name, color = Color.Black.copy(alpha = 0.75f), fontWeight = FontWeight.SemiBold, fontSize = 11.sp)
                if (player.mulligans > 0) {
                    Text(
                        stringResource(R.string.tracker_mulligans_suffix, player.mulligans),
                        color = Color.Black.copy(alpha = 0.55f),
                        fontSize = 9.sp
                    )
                }
            }

            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                LifeStepButton("−", onClick = { onLifeChange(-1) })
                Text(player.life.toString(), color = Color.Black, fontWeight = FontWeight.Bold, fontSize = 38.sp)
                LifeStepButton("+", onClick = { onLifeChange(1) })
            }

            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.heightIn(min = 14.dp)
            ) {
                if (player.poison > 0) {
                    Text("☠ ${player.poison}", color = StatusPoison.copy(alpha = 0.9f), fontSize = 10.sp)
                }
                opponents.forEach { opponent ->
                    val damage = player.commanderDamage[opponent.id] ?: 0
                    if (damage > 0) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Box(Modifier.size(7.dp).clip(CircleShape).background(opponent.color))
                            Spacer(Modifier.width(3.dp))
                            Text(damage.toString(), color = Color.Black.copy(alpha = 0.65f), fontSize = 10.sp)
                        }
                    }
                }
            }

            Box(
                modifier = Modifier
                    .clip(RoundedCornerShape(percent = 50))
                    .border(1.dp, Color.Black.copy(alpha = 0.3f), RoundedCornerShape(percent = 50))
                    .background(Color.White.copy(alpha = 0.2f))
                    .clickable { onPassTurn() }
                    .padding(horizontal = 14.dp, vertical = 5.dp)
            ) {
                Text(
                    stringResource(R.string.tracker_pass_turn),
                    color = Color.Black.copy(alpha = 0.75f),
                    fontWeight = FontWeight.SemiBold,
                    fontSize = 10.sp
                )
            }
        }

        if (expanded) {
            CommanderDamagePanel(
                opponents = opponents,
                commanderDamage = player.commanderDamage,
                poison = player.poison,
                onCommanderDamageChange = onCommanderDamageChange,
                onPoisonChange = onPoisonChange,
                modifier = Modifier.fillMaxSize().clickable(onClick = onToggleExpand)
            )
        }

        if (deathAlpha > 0f) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(Color.Black.copy(alpha = 0.9f * deathAlpha)),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    stringResource(R.string.tracker_dead),
                    color = Color.White.copy(alpha = deathAlpha),
                    fontWeight = FontWeight.Light,
                    fontSize = 19.sp,
                    letterSpacing = 2.sp
                )
            }
        }
    }
}

@Composable
private fun LifeStepButton(label: String, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .size(32.dp)
            .clip(CircleShape)
            .background(Color.Black.copy(alpha = 0.12f))
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Text(label, color = Color.Black, fontWeight = FontWeight.Bold, fontSize = 17.sp)
    }
}

@Composable
private fun CommanderDamagePanel(
    opponents: List<PlayerState>,
    commanderDamage: Map<Int, Int>,
    poison: Int,
    onCommanderDamageChange: (attackerId: Int, delta: Int) -> Unit,
    onPoisonChange: (Int) -> Unit,
    modifier: Modifier = Modifier
) {
    Box(
        modifier = modifier.background(Color(0xF7050308)),
        contentAlignment = Alignment.Center
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(5.dp)) {
            Text(stringResource(R.string.tracker_commander_damage), color = AccentSoft, fontSize = 9.sp, letterSpacing = 0.5.sp)
            opponents.forEach { opponent ->
                val amount = commanderDamage[opponent.id] ?: 0
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Box(Modifier.size(10.dp).clip(CircleShape).background(opponent.color))
                    MiniStepButton("−") { onCommanderDamageChange(opponent.id, -1) }
                    Text(
                        amount.toString(),
                        color = Color.White,
                        fontSize = 11.sp,
                        textAlign = TextAlign.Center,
                        modifier = Modifier.width(16.dp)
                    )
                    MiniStepButton("+") { onCommanderDamageChange(opponent.id, 1) }
                }
            }
            Spacer(Modifier.height(4.dp))
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                Text(stringResource(R.string.tracker_poison), color = StatusPoison, fontSize = 9.sp)
                MiniStepButton("−") { onPoisonChange(-1) }
                Text(
                    poison.toString(),
                    color = Color.White,
                    fontSize = 11.sp,
                    textAlign = TextAlign.Center,
                    modifier = Modifier.width(16.dp)
                )
                MiniStepButton("+") { onPoisonChange(1) }
            }
        }
    }
}

@Composable
private fun MiniStepButton(label: String, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .size(18.dp)
            .clip(CircleShape)
            .background(Color.White.copy(alpha = 0.12f))
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Text(label, color = Color.White, fontSize = 10.sp)
    }
}

@Composable
private fun PauseButton(onClick: () -> Unit, modifier: Modifier = Modifier) {
    Box(
        modifier = modifier
            .size(56.dp)
            .clip(CircleShape)
            .background(AppBackgroundDeep.copy(alpha = 0.85f))
            .border(1.dp, AccentSoft.copy(alpha = 0.4f), CircleShape)
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            Box(Modifier.width(4.dp).height(16.dp).clip(RoundedCornerShape(2.dp)).background(AccentSoft))
            Box(Modifier.width(4.dp).height(16.dp).clip(RoundedCornerShape(2.dp)).background(AccentSoft))
        }
    }
}

@Composable
private fun StarterBanner(name: String, modifier: Modifier = Modifier) {
    Box(modifier = modifier.background(Color(0xF7050308)), contentAlignment = Alignment.Center) {
        Text(
            text = stringResource(R.string.tracker_starter_banner, name),
            color = AppOnBackground,
            fontWeight = FontWeight.Bold,
            fontSize = 22.sp,
            textAlign = TextAlign.Center
        )
    }
}

@Composable
private fun PauseOverlay(onResume: () -> Unit, onEnd: () -> Unit, modifier: Modifier = Modifier) {
    Box(modifier = modifier.background(Color(0xF7050308)), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(14.dp)) {
            Text(stringResource(R.string.tracker_paused), color = AppOnBackground, fontWeight = FontWeight.SemiBold, fontSize = 16.sp)
            Text(stringResource(R.string.tracker_paused_subtitle), color = AppFaint, fontSize = 12.sp)
            GradientButton(text = stringResource(R.string.tracker_resume), onClick = onResume, modifier = Modifier.width(180.dp))
            Text(
                stringResource(R.string.tracker_finish_game),
                color = StatusDanger,
                fontSize = 13.sp,
                modifier = Modifier.clickable(onClick = onEnd)
            )
        }
    }
}

private val SummaryColumnWeights = listOf(1.3f, 0.7f, 0.9f, 0.9f, 0.7f, 0.8f, 0.9f)

@Composable
private fun GameSummary(state: GameState, onBack: () -> Unit) {
    val winner = state.players.firstOrNull { it.id == state.winnerId }
    val summaryColumnLabels = listOf(
        stringResource(R.string.summary_column_player),
        stringResource(R.string.summary_column_life),
        stringResource(R.string.summary_column_dealt),
        stringResource(R.string.summary_column_taken),
        stringResource(R.string.summary_column_poison),
        stringResource(R.string.summary_column_mulligans),
        stringResource(R.string.summary_column_status)
    )
    Column(
        modifier = Modifier.fillMaxSize().padding(horizontal = 26.dp, vertical = 18.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Text(
            stringResource(R.string.summary_subtitle, state.currentTurn),
            color = AppFaint,
            fontSize = 11.sp,
            letterSpacing = 0.5.sp
        )
        Spacer(Modifier.height(10.dp))

        if (winner != null) {
            Box(
                modifier = Modifier
                    .clip(RoundedCornerShape(18.dp, 12.dp, 18.dp, 12.dp))
                    .background(winner.color)
                    .padding(horizontal = 20.dp, vertical = 8.dp)
            ) {
                Text(
                    stringResource(R.string.summary_winner_banner, winner.name),
                    color = Color.Black,
                    fontWeight = FontWeight.Bold,
                    fontSize = 14.sp
                )
            }
            Spacer(Modifier.height(14.dp))
        }

        Column(
            modifier = Modifier.weight(1f).fillMaxWidth().verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(6.dp)
        ) {
            Row(modifier = Modifier.fillMaxWidth().padding(horizontal = 10.dp), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                summaryColumnLabels.forEachIndexed { index, label ->
                    Text(label, color = AppFaint, fontSize = 9.sp, modifier = Modifier.weight(SummaryColumnWeights[index]))
                }
            }
            state.players.forEach { player ->
                val dealt = state.players.sumOf { it.commanderDamage[player.id] ?: 0 }
                val taken = player.commanderDamage.values.maxOrNull() ?: 0
                val isWinner = winner?.id == player.id
                SummaryRow(player = player, dealt = dealt, taken = taken, isWinner = isWinner)
            }
        }

        Spacer(Modifier.height(12.dp))
        GradientButton(text = stringResource(R.string.summary_back_home), onClick = onBack)
    }
}

@Composable
private fun SummaryRow(player: PlayerState, dealt: Int, taken: Int, isWinner: Boolean) {
    val statusLabel = when {
        isWinner -> stringResource(R.string.summary_status_winner)
        player.isEliminated() -> stringResource(R.string.summary_status_eliminated)
        else -> stringResource(R.string.summary_status_in_play)
    }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .background(Color.White.copy(alpha = 0.03f))
            .border(1.dp, AppOutline, RoundedCornerShape(12.dp))
            .padding(horizontal = 10.dp, vertical = 7.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(4.dp)
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.weight(SummaryColumnWeights[0])) {
            Box(Modifier.size(9.dp).clip(CircleShape).background(player.color))
            Spacer(Modifier.width(6.dp))
            Text(player.name, color = AppOnBackground, fontSize = 11.sp)
        }
        Text(player.life.toString(), color = AppMuted, fontSize = 11.sp, modifier = Modifier.weight(SummaryColumnWeights[1]))
        Text(dealt.toString(), color = AppMuted, fontSize = 11.sp, modifier = Modifier.weight(SummaryColumnWeights[2]))
        Text(taken.toString(), color = AppMuted, fontSize = 11.sp, modifier = Modifier.weight(SummaryColumnWeights[3]))
        Text(player.poison.toString(), color = AppMuted, fontSize = 11.sp, modifier = Modifier.weight(SummaryColumnWeights[4]))
        Text(player.mulligans.toString(), color = AppMuted, fontSize = 11.sp, modifier = Modifier.weight(SummaryColumnWeights[5]))
        Text(statusLabel, color = AccentSoft, fontWeight = FontWeight.SemiBold, fontSize = 10.sp, modifier = Modifier.weight(SummaryColumnWeights[6]))
    }
}

/**
 * Informational banner for the backend sync status.
 *
 * Deliberately non-blocking: the game plays the same locally either way, so it's only shown
 * when there's something to report (not on [RemoteSyncStatus.Synced], the silent case).
 */
@Composable
private fun RemoteSyncBanner(remoteSync: RemoteSyncState, modifier: Modifier = Modifier) {
    val label = when (remoteSync.status) {
        RemoteSyncStatus.Connecting -> stringResource(R.string.tracker_connecting_to_server)
        RemoteSyncStatus.Synced -> null
        RemoteSyncStatus.Disabled,
        RemoteSyncStatus.WaitingForPlayers,
        RemoteSyncStatus.Failed -> remoteSync.message
    } ?: return

    val isError = remoteSync.status == RemoteSyncStatus.Failed
    Box(
        modifier = modifier
            .clip(RoundedCornerShape(percent = 50))
            .background(if (isError) StatusDanger.copy(alpha = 0.18f) else Color.White.copy(alpha = 0.06f))
            .padding(horizontal = 10.dp, vertical = 4.dp)
    ) {
        Text(
            text = label,
            color = if (isError) StatusDanger else AppFaint,
            fontSize = 9.sp
        )
    }
}
