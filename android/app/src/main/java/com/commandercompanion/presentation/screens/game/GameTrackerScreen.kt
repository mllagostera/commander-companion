package com.commandercompanion.presentation.screens.game

import android.content.res.Configuration
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.LocalTextStyle
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shadow
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
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
import kotlin.math.cos
import kotlin.math.roundToInt
import kotlin.math.sin
import kotlinx.coroutines.delay

/** How long the starter-draw ring spins across the seats before landing, and how fast each step advances. */
private const val RANDOMIZE_STEP_DELAY_MS = 130L
private const val RANDOMIZE_STEPS = 10

/** How long one full lap of the orbiting turn label takes around the pause button. */
private const val ORBIT_SPIN_DURATION_MS = 9000
private val ORBIT_RADIUS = 62.dp

/** One pulse of the red rim that flashes over an eliminated seat. */
private const val ELIMINATION_FLASH_PERIOD_MS = 1600

private val QuadrantShape = RoundedCornerShape(22.dp)

/** Overlay scrims, taken from the design: expanded panel and starter banner, pause, and summary. */
private val OverlayScrim = Color(0xD9050308)
private val PauseScrim = Color(0xEB050308)
private val SummaryScrim = Color(0xF7050308)

@Composable
fun GameTrackerScreen(
    onFinish: () -> Unit,
    viewModel: GameViewModel = hiltViewModel()
) {
    val state by viewModel.state
    var paused by rememberSaveable { mutableStateOf(false) }
    var expandedPlayerId by rememberSaveable { mutableStateOf<Int?>(null) }
    var randomizingStarter by rememberSaveable { mutableStateOf(state.startingPlayerId != null) }
    var randomHighlightId by rememberSaveable { mutableStateOf<Int?>(null) }
    var showStarterBanner by rememberSaveable { mutableStateOf(false) }

    LaunchedEffect(Unit) {
        if (!randomizingStarter) return@LaunchedEffect
        val seatIds = state.players.map { it.id }
        if (seatIds.isNotEmpty()) {
            repeat(RANDOMIZE_STEPS) { step ->
                randomHighlightId = seatIds[step % seatIds.size]
                delay(RANDOMIZE_STEP_DELAY_MS)
            }
        }
        randomizingStarter = false
        randomHighlightId = null
        showStarterBanner = true
        delay(1800)
        showStarterBanner = false
    }

    val isLandscape = LocalConfiguration.current.orientation == Configuration.ORIENTATION_LANDSCAPE

    Box(modifier = Modifier.fillMaxSize().background(AppBackgroundDeep)) {
        when {
            !isLandscape -> RotateDevicePrompt(message = stringResource(R.string.tracker_rotate_prompt))
            state.isFinished -> GameSummary(state = state, onBack = onFinish)
            state.players.isEmpty() -> LoadingTable(remoteSync = state.remoteSync, onBack = onFinish)
            else -> {
                QuadrantGrid(
                    players = state.players,
                    localSeatId = state.localSeatId,
                    expandedPlayerId = expandedPlayerId,
                    onToggleExpand = { id -> expandedPlayerId = if (expandedPlayerId == id) null else id },
                    onLifeChange = viewModel::adjustLife,
                    onCommanderDamageChange = viewModel::adjustCommanderDamage,
                    onPoisonChange = viewModel::adjustPoison,
                    onPassTurn = { viewModel.nextTurn() },
                    activeTurnPlayerId = state.currentTurnPlayerId,
                    randomizingStarter = randomizingStarter,
                    randomHighlightId = randomHighlightId
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

                OrbitingTurnLabel(turn = state.currentTurn, modifier = Modifier.align(Alignment.Center))

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
                        // Resetting the whole table only makes sense when this device is
                        // authoritative for every seat (pass-and-play host mode) — in joined mode
                        // it only owns its own seat, see [GameState.localSeatId].
                        onResetLives = { viewModel.resetLives() }.takeIf { state.localSeatId == null },
                        onEnd = { viewModel.finishGame() },
                        modifier = Modifier.matchParentSize()
                    )
                }
            }
        }
    }
}

/**
 * Splits the table the way it is seated: the first half sits "at the top" (rotated 180°), the rest
 * below. Both the seat grid and each seat's commander-damage grid use this, so a seat occupies the
 * same relative position in either — which is what makes the damage grid readable at a glance.
 */
private fun seatRows(players: List<PlayerState>): Pair<List<PlayerState>, List<PlayerState>> {
    val topCount = (players.size + 1) / 2
    return players.take(topCount) to players.drop(topCount)
}

/** Seat grid: first half "at the top of the table" (rotated 180°), the rest below. Works for 2-6. */
@Composable
private fun QuadrantGrid(
    players: List<PlayerState>,
    localSeatId: Int?,
    expandedPlayerId: Int?,
    onToggleExpand: (Int) -> Unit,
    onLifeChange: (playerId: Int, amount: Int) -> Unit,
    onCommanderDamageChange: (targetPlayerId: Int, attackerId: Int, amount: Int) -> Unit,
    onPoisonChange: (playerId: Int, amount: Int) -> Unit,
    onPassTurn: () -> Unit,
    activeTurnPlayerId: Int?,
    randomizingStarter: Boolean,
    randomHighlightId: Int?
) {
    val (topSeats, bottomSeats) = seatRows(players)
    // While the starter draw is spinning, its highlight takes over the ring from whoever's turn it
    // actually is; once it lands, the ring reverts to reflecting the real turn owner.
    fun ringFor(playerId: Int): SeatRing? = when {
        randomizingStarter && randomHighlightId == playerId -> SeatRing.Randomizing
        !randomizingStarter && activeTurnPlayerId == playerId -> SeatRing.ActiveTurn
        else -> null
    }

    Column(
        modifier = Modifier.fillMaxSize().padding(4.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp)
    ) {
        listOf(topSeats to true, bottomSeats to false).forEach { (seats, rotated) ->
            Row(
                modifier = Modifier.weight(1f).fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                seats.forEach { player ->
                    // Null localSeatId = pass-and-play (host mode): every seat on this one device is
                    // editable, as it's always been. Non-null = joined mode: only the local seat is.
                    val editable = localSeatId == null || player.id == localSeatId
                    PlayerQuadrant(
                        player = player,
                        table = players,
                        editable = editable,
                        expanded = expandedPlayerId == player.id,
                        onToggleExpand = { onToggleExpand(player.id) },
                        onLifeChange = { delta -> onLifeChange(player.id, delta) },
                        onCommanderDamageChange = { attackerId, delta -> onCommanderDamageChange(player.id, attackerId, delta) },
                        onPoisonChange = { delta -> onPoisonChange(player.id, delta) },
                        onPassTurn = onPassTurn,
                        ring = ringFor(player.id),
                        rotated = rotated,
                        modifier = Modifier.weight(1f).fillMaxHeight()
                    )
                }
            }
        }
    }
}

/** Which ring, if any, wraps a seat's quadrant this frame — see [QuadrantGrid]'s `ringFor`. */
private enum class SeatRing { Randomizing, ActiveTurn }

/**
 * Text and ornament colors for one seat.
 *
 * The design paints white text over its violet seats, but the shipped seat palette is the lighter
 * mana one ([com.commandercompanion.presentation.theme.PlayerColorPalette]), where white is
 * illegible — so the pair is derived from the seat's own luminance instead of hard-coded.
 */
private data class SeatInk(
    val primary: Color,
    val secondary: Color,
    val cell: Color,
    val onLightSeat: Boolean
)

private fun inkFor(seat: Color): SeatInk = if (seat.luminance() > 0.35f) {
    SeatInk(
        primary = Color.Black.copy(alpha = 0.85f),
        secondary = Color.Black.copy(alpha = 0.6f),
        cell = Color.Black.copy(alpha = 0.16f),
        onLightSeat = true
    )
} else {
    SeatInk(
        primary = Color.White,
        secondary = Color.White.copy(alpha = 0.72f),
        cell = Color.White.copy(alpha = 0.22f),
        onLightSeat = false
    )
}

/** Drop shadow that keeps the life total readable over a dark seat, as in the design. */
private val SeatTextShadow = Shadow(color = Color(0x800A0714), offset = Offset(0f, 2f), blurRadius = 6f)

@Composable
private fun PlayerQuadrant(
    player: PlayerState,
    table: List<PlayerState>,
    editable: Boolean,
    expanded: Boolean,
    onToggleExpand: () -> Unit,
    onLifeChange: (Int) -> Unit,
    onCommanderDamageChange: (attackerId: Int, delta: Int) -> Unit,
    onPoisonChange: (Int) -> Unit,
    onPassTurn: () -> Unit,
    ring: SeatRing?,
    rotated: Boolean,
    modifier: Modifier = Modifier
) {
    val eliminated = player.isEliminated()
    val deathAlpha by animateFloatAsState(targetValue = if (eliminated) 1f else 0f, animationSpec = tween(900), label = "death")
    val ink = inkFor(player.color)
    val ringColor = when (ring) {
        SeatRing.Randomizing -> AppOnBackground
        SeatRing.ActiveTurn -> AccentSoft
        null -> null
    }

    Box(
        modifier = modifier
            .then(if (rotated) Modifier.rotate(180f) else Modifier)
            // The active seat's ring glows; the starter-draw one is a plain white outline.
            .then(
                if (ring == SeatRing.ActiveTurn) {
                    Modifier.shadow(14.dp, QuadrantShape, ambientColor = AccentSoft, spotColor = AccentSoft)
                } else {
                    Modifier
                }
            )
            .clip(QuadrantShape)
            .background(player.color)
            .then(if (ringColor != null) Modifier.border(4.dp, ringColor, QuadrantShape) else Modifier)
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(Brush.verticalGradient(listOf(Color(0x0D0A0714), Color(0x730A0714))))
        )

        // Life is adjusted by tapping either half of the seat — the whole quadrant is the control,
        // the ± glyphs below are only decoration. Read-only seats (joined mode) get no tap zones.
        if (editable) {
            Row(modifier = Modifier.fillMaxSize()) {
                LifeTapZone(
                    onClick = { onLifeChange(-1) },
                    label = stringResource(R.string.tracker_life_decrease),
                    modifier = Modifier.weight(1f).fillMaxHeight()
                )
                LifeTapZone(
                    onClick = { onLifeChange(1) },
                    label = stringResource(R.string.tracker_life_increase),
                    modifier = Modifier.weight(1f).fillMaxHeight()
                )
            }
            LifeStepGlyph("−", Modifier.align(Alignment.CenterStart).padding(start = 8.dp))
            LifeStepGlyph("+", Modifier.align(Alignment.CenterEnd).padding(end = 8.dp))
        }

        // Not clickable itself, so taps that miss its buttons fall through to the tap zones above.
        Column(
            modifier = Modifier.fillMaxSize().padding(horizontal = 12.dp, vertical = 10.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.SpaceBetween
        ) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Text(
                    player.name,
                    color = ink.primary.copy(alpha = 0.9f),
                    fontWeight = FontWeight.SemiBold,
                    fontSize = 11.sp,
                    style = LocalTextStyle.current.copy(
                        shadow = SeatTextShadow.takeIf { !ink.onLightSeat }
                    )
                )
                if (player.mulligans > 0) {
                    Text(
                        stringResource(R.string.tracker_mulligans_suffix, player.mulligans),
                        color = ink.secondary,
                        fontSize = 9.sp
                    )
                }
            }

            Text(
                player.life.toString(),
                color = ink.primary,
                fontWeight = FontWeight.Bold,
                fontSize = 38.sp,
                textAlign = TextAlign.Center,
                modifier = Modifier.widthIn(min = 56.dp),
                style = LocalTextStyle.current.copy(
                    shadow = SeatTextShadow.takeIf { !ink.onLightSeat }
                )
            )

            CommanderDamageMiniGrid(
                player = player,
                table = table,
                ink = ink,
                onClick = onToggleExpand
            )

            Box(
                modifier = Modifier
                    .clip(CircleShape)
                    .border(1.dp, ink.primary.copy(alpha = 0.35f), CircleShape)
                    .background(Color.Black.copy(alpha = 0.25f))
                    .clickable { onPassTurn() }
                    .padding(horizontal = 14.dp, vertical = 5.dp)
            ) {
                Text(
                    stringResource(R.string.tracker_pass_turn),
                    color = if (ink.onLightSeat) ink.primary else Color.White,
                    fontWeight = FontWeight.SemiBold,
                    fontSize = 10.sp
                )
            }
        }

        if (expanded) {
            CommanderDamagePanel(
                player = player,
                table = table,
                editable = editable,
                onCommanderDamageChange = onCommanderDamageChange,
                onPoisonChange = onPoisonChange,
                onDismiss = onToggleExpand,
                modifier = Modifier.fillMaxSize()
            )
        }

        if (deathAlpha > 0f) {
            EliminationOverlay(alpha = deathAlpha, modifier = Modifier.fillMaxSize())
        }
    }
}

/** Half of a seat: tapping it steps life, and flashes so the player sees which half registered. */
@Composable
private fun LifeTapZone(onClick: () -> Unit, label: String, modifier: Modifier = Modifier) {
    val interactionSource = remember { MutableInteractionSource() }
    val pressed by interactionSource.collectIsPressedAsState()
    val flash by animateFloatAsState(
        targetValue = if (pressed) 0.16f else 0f,
        animationSpec = tween(durationMillis = if (pressed) 0 else 260),
        label = "life-tap-flash"
    )
    Box(
        modifier = modifier
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                onClickLabel = label,
                onClick = onClick
            )
            .background(Color.White.copy(alpha = flash))
    )
}

/** The oversized ± at each edge of a seat: decoration for the tap zones, not a button. */
@Composable
private fun LifeStepGlyph(symbol: String, modifier: Modifier = Modifier) {
    Text(
        text = symbol,
        color = Color.Black.copy(alpha = 0.38f),
        fontWeight = FontWeight.Light,
        fontSize = 42.sp,
        modifier = modifier
    )
}

/**
 * Always-visible summary of the commander damage this seat has taken, laid out like the table
 * itself (see [seatRows]) so each opponent sits where they are actually sitting. The seat's own
 * cell shows a person mark instead of a number, and poison hangs underneath when there is any.
 * Tapping the block opens [CommanderDamagePanel].
 */
@Composable
private fun CommanderDamageMiniGrid(
    player: PlayerState,
    table: List<PlayerState>,
    ink: SeatInk,
    onClick: () -> Unit
) {
    val (topSeats, bottomSeats) = seatRows(table)

    Column(
        modifier = Modifier
            .clip(RoundedCornerShape(10.dp))
            .background(Color.Black.copy(alpha = 0.22f))
            .clickable(onClick = onClick)
            .padding(5.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(3.dp)
    ) {
        MiniDamageRow(seats = topSeats, player = player, ink = ink)
        MiniDamageRow(seats = bottomSeats, player = player, ink = ink)
        if (player.poison > 0) {
            Text("☠ ${player.poison}", color = StatusPoison, fontSize = 9.sp)
        }
    }
}

/** One row of the mini grid: the seat's own cell carries a person mark, the rest their damage. */
@Composable
private fun MiniDamageRow(seats: List<PlayerState>, player: PlayerState, ink: SeatInk) {
    Row(horizontalArrangement = Arrangement.spacedBy(3.dp)) {
        seats.forEach { seat ->
            Box(
                modifier = Modifier.size(22.dp).clip(RoundedCornerShape(6.dp)).background(ink.cell),
                contentAlignment = Alignment.Center
            ) {
                if (seat.id == player.id) {
                    SelfMark(color = ink.primary.copy(alpha = 0.85f), scale = 0.6f)
                } else {
                    Text(
                        (player.commanderDamage[seat.id] ?: 0).toString(),
                        color = ink.primary,
                        fontWeight = FontWeight.Bold,
                        fontSize = 11.sp
                    )
                }
            }
        }
    }
}

/** Head-and-shoulders mark that stands in for "this is you" in the commander-damage grids. */
@Composable
private fun SelfMark(color: Color, scale: Float) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Box(Modifier.size(11.dp * scale).clip(CircleShape).background(color))
        Box(
            Modifier
                .padding(top = 1.dp * scale)
                .size(width = 18.dp * scale, height = 9.dp * scale)
                .clip(RoundedCornerShape(topStart = 9.dp, topEnd = 9.dp))
                .background(color)
        )
    }
}

/**
 * Expanded editor over a seat: one cell per opponent, in the table's own layout, plus the poison
 * row. Tapping anywhere outside the controls closes it — without that there is no way back to the
 * board on a phone.
 */
@Composable
private fun CommanderDamagePanel(
    player: PlayerState,
    table: List<PlayerState>,
    editable: Boolean,
    onCommanderDamageChange: (attackerId: Int, delta: Int) -> Unit,
    onPoisonChange: (Int) -> Unit,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier
) {
    val (topSeats, bottomSeats) = seatRows(table)

    Box(
        modifier = modifier
            .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null, onClick = onDismiss)
            .background(OverlayScrim),
        contentAlignment = Alignment.Center
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(
                stringResource(R.string.tracker_commander_damage),
                color = AccentSoft,
                fontSize = 9.sp,
                letterSpacing = 0.5.sp
            )
            DamageEditorRow(
                seats = topSeats,
                player = player,
                editable = editable,
                onCommanderDamageChange = onCommanderDamageChange
            )
            DamageEditorRow(
                seats = bottomSeats,
                player = player,
                editable = editable,
                onCommanderDamageChange = onCommanderDamageChange
            )
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(6.dp),
                modifier = Modifier
                    .padding(top = 2.dp)
                    .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null) {}
            ) {
                Text(stringResource(R.string.tracker_poison), color = StatusPoison, fontSize = 9.sp)
                if (editable) MiniStepButton("−") { onPoisonChange(-1) }
                Text(
                    player.poison.toString(),
                    color = Color.White,
                    fontSize = 11.sp,
                    textAlign = TextAlign.Center,
                    modifier = Modifier.width(16.dp)
                )
                if (editable) MiniStepButton("+") { onPoisonChange(1) }
            }
        }
    }
}

/** One row of the expanded editor: a coloured cell per seat, with ± for every opponent. */
@Composable
private fun DamageEditorRow(
    seats: List<PlayerState>,
    player: PlayerState,
    editable: Boolean,
    onCommanderDamageChange: (attackerId: Int, delta: Int) -> Unit
) {
    Row(
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        // Swallows taps between cells so a miss does not dismiss the panel mid-edit.
        modifier = Modifier.clickable(
            interactionSource = remember { MutableInteractionSource() },
            indication = null
        ) {}
    ) {
        seats.forEach { seat ->
            Box(
                modifier = Modifier.size(44.dp).clip(RoundedCornerShape(10.dp)).background(seat.color),
                contentAlignment = Alignment.Center
            ) {
                if (seat.id == player.id) {
                    SelfMark(color = Color.Black.copy(alpha = 0.5f), scale = 1f)
                } else {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(2.dp)) {
                        if (editable) MiniStepButton("−") { onCommanderDamageChange(seat.id, -1) }
                        Text(
                            (player.commanderDamage[seat.id] ?: 0).toString(),
                            color = Color.Black,
                            fontWeight = FontWeight.Bold,
                            fontSize = 12.sp
                        )
                        if (editable) MiniStepButton("+") { onCommanderDamageChange(seat.id, 1) }
                    }
                }
            }
        }
    }
}

@Composable
private fun MiniStepButton(label: String, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .size(16.dp)
            .clip(CircleShape)
            .background(Color.Black.copy(alpha = 0.2f))
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Text(label, color = Color.Black, fontSize = 10.sp)
    }
}

/** Fades a dead seat to black with a skull and a red rim that keeps pulsing while it is out. */
@Composable
private fun EliminationOverlay(alpha: Float, modifier: Modifier = Modifier) {
    val transition = rememberInfiniteTransition(label = "elimination")
    val flash by transition.animateFloat(
        initialValue = 0.35f,
        targetValue = 0.85f,
        animationSpec = infiniteRepeatable(
            animation = tween(ELIMINATION_FLASH_PERIOD_MS / 2, easing = LinearEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "elimination-flash"
    )

    Box(modifier = modifier.background(Color(0xFF0A0000).copy(alpha = 0.72f * alpha))) {
        // Red only at the edges, fading out towards the middle, so the skull stays readable.
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(
                    Brush.radialGradient(
                        0.38f to Color.Transparent,
                        1f to Color(0xFFDC1414).copy(alpha = 0.55f * flash * alpha)
                    )
                )
        )
        Column(
            modifier = Modifier.fillMaxSize(),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            Text("💀", fontSize = 34.sp)
            Spacer(Modifier.height(4.dp))
            Text(
                stringResource(R.string.tracker_dead),
                color = AppOnBackground.copy(alpha = alpha),
                fontWeight = FontWeight.ExtraBold,
                fontSize = 15.sp,
                letterSpacing = 1.5.sp,
                style = LocalTextStyle.current.copy(
                    shadow = Shadow(color = Color.Black, offset = Offset(1f, 1f), blurRadius = 0f)
                )
            )
        }
    }
}

/** Spells "Turno N" as a ring of characters slowly orbiting the pause button. */
@Composable
private fun OrbitingTurnLabel(turn: Int, modifier: Modifier = Modifier) {
    val label = stringResource(R.string.tracker_turn_label, turn)
    val infiniteTransition = rememberInfiniteTransition(label = "orbit")
    val spinAngle by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 360f,
        animationSpec = infiniteRepeatable(animation = tween(ORBIT_SPIN_DURATION_MS, easing = LinearEasing)),
        label = "orbit-angle"
    )
    val radiusPx = with(LocalDensity.current) { ORBIT_RADIUS.toPx() }
    val arc = minOf(160f, label.length * 18f)
    val start = -arc / 2f
    val step = if (label.length > 1) arc / (label.length - 1) else 0f

    Box(modifier = modifier.graphicsLayer { rotationZ = spinAngle }) {
        label.forEachIndexed { i, ch ->
            val angle = Math.toRadians((start + i * step).toDouble())
            val x = (radiusPx * sin(angle)).roundToInt()
            val y = (-radiusPx * cos(angle)).roundToInt()
            Text(
                text = ch.toString(),
                color = Color.White.copy(alpha = 0.9f),
                fontWeight = FontWeight.Bold,
                fontSize = 9.sp,
                modifier = Modifier.offset { IntOffset(x, y) }
            )
        }
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
    Box(modifier = modifier.background(OverlayScrim), contentAlignment = Alignment.Center) {
        Text(
            text = stringResource(R.string.tracker_starter_banner, name),
            color = AppOnBackground,
            fontWeight = FontWeight.Bold,
            fontSize = 22.sp,
            textAlign = TextAlign.Center
        )
    }
}

/**
 * Shown instead of the [QuadrantGrid] while joined mode is still fetching the rest of the table
 * (`GameViewModel.initJoinedGame`) — pass-and-play mode never hits this, its players are known
 * synchronously from `playersEncoded`. On failure, offers a way back instead of loading forever.
 */
@Composable
private fun LoadingTable(remoteSync: RemoteSyncState, onBack: () -> Unit) {
    val isError = remoteSync.status == RemoteSyncStatus.Failed
    Column(
        modifier = Modifier.fillMaxSize().padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Text(
            text = remoteSync.message ?: stringResource(R.string.tracker_loading_joined_game),
            color = if (isError) StatusDanger else AppOnBackground,
            fontSize = 14.sp,
            textAlign = TextAlign.Center
        )
        if (isError) {
            Spacer(Modifier.height(16.dp))
            GradientButton(text = stringResource(R.string.tracker_back_to_dashboard), onClick = onBack)
        }
    }
}

@Composable
private fun PauseOverlay(
    onResume: () -> Unit,
    onResetLives: (() -> Unit)?,
    onEnd: () -> Unit,
    modifier: Modifier = Modifier
) {
    Box(modifier = modifier.background(PauseScrim), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(14.dp)) {
            Text(stringResource(R.string.tracker_paused), color = AppOnBackground, fontWeight = FontWeight.SemiBold, fontSize = 16.sp)
            Text(stringResource(R.string.tracker_paused_subtitle), color = AppFaint, fontSize = 12.sp)
            GradientButton(text = stringResource(R.string.tracker_resume), onClick = onResume, modifier = Modifier.width(180.dp))
            if (onResetLives != null) {
                Text(
                    stringResource(R.string.tracker_reset_lives),
                    color = StatusDanger,
                    fontSize = 13.sp,
                    modifier = Modifier.clickable(onClick = onResetLives)
                )
            }
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
        modifier = Modifier.fillMaxSize().background(SummaryScrim).padding(horizontal = 26.dp, vertical = 18.dp),
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
