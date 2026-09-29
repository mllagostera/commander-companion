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
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shadow
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.LayoutCoordinates
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import coil3.compose.AsyncImage
import com.commandercompanion.R
import com.commandercompanion.presentation.components.GradientButton
import com.commandercompanion.presentation.components.KeepScreenOn
import com.commandercompanion.presentation.components.RotateDevicePrompt
import com.commandercompanion.presentation.components.message
import com.commandercompanion.presentation.theme.AccentGradient
import com.commandercompanion.presentation.theme.AccentSoft
import com.commandercompanion.presentation.theme.AppBackgroundDeep
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppMuted
import com.commandercompanion.presentation.theme.AppOnBackground
import com.commandercompanion.presentation.theme.AppOutline
import com.commandercompanion.presentation.theme.StatusDanger
import com.commandercompanion.presentation.theme.StatusPoison
import java.util.Locale
import kotlinx.coroutines.delay

/** How long the starter-draw ring spins across the seats before landing, and how fast each step advances. */
private const val RANDOMIZE_STEP_DELAY_MS = 130L
private const val RANDOMIZE_STEPS = 10

/** One pulse of the red rim that flashes over an eliminated seat. */
private const val ELIMINATION_FLASH_PERIOD_MS = 1600

private val QuadrantShape = RoundedCornerShape(22.dp)

/** The turn owner's pass-turn bar: a full touch target, not a caption. */
private val PassTurnBarHeight = 48.dp

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
    // The seats of a finished damage drag, while its dialog picks the amount.
    var damageSourceId by rememberSaveable { mutableStateOf<Int?>(null) }
    var damageTargetId by rememberSaveable { mutableStateOf<Int?>(null) }

    // The phone lies on the table for the whole game: the screen must not time out between taps.
    // Scoped to this screen (not the Activity window) so the rest of the app keeps the system timeout.
    KeepScreenOn()

    LaunchedEffect(Unit) {
        // The first turn -- and with it the turn clocks -- starts once the starter draw lands.
        // Restored after a rotation, the draw is already over and this is a no-op.
        if (!randomizingStarter) {
            viewModel.startTurnClock()
            return@LaunchedEffect
        }
        // Spins around the table the same way the turn will (see [clockwiseSeats]), so the ring
        // never jumps diagonally across the quadrants while the starter is being drawn.
        val seatIds = clockwiseSeats(state.players).map { it.id }
        if (seatIds.isNotEmpty()) {
            repeat(RANDOMIZE_STEPS) { step ->
                randomHighlightId = seatIds[step % seatIds.size]
                delay(RANDOMIZE_STEP_DELAY_MS)
            }
        }
        randomizingStarter = false
        randomHighlightId = null
        viewModel.startTurnClock()
        showStarterBanner = true
        delay(1800)
        showStarterBanner = false
    }

    // Pausing the game stops the turn clock; resuming picks it up where it was.
    LaunchedEffect(paused) {
        if (paused) viewModel.pauseTurnClock() else viewModel.resumeTurnClock()
    }

    val isLandscape = LocalConfiguration.current.orientation == Configuration.ORIENTATION_LANDSCAPE

    // Edge-to-edge: the deep violet fills the whole window, and the safe-area padding is applied
    // per child rather than on this Box. Anything the player *touches* is inset -- in landscape the
    // safe area excludes the side gesture bar and any display cutout, which would otherwise swallow
    // taps on the life counters at the screen edges. The full-screen scrims (pause, starter banner)
    // deliberately stay full-bleed: a modal that stopped at the safe area would leave an
    // un-dimmed strip under the status bar and the gesture bar.
    Box(modifier = Modifier.fillMaxSize().background(AppBackgroundDeep)) {
        when {
            !isLandscape -> RotateDevicePrompt(
                message = stringResource(R.string.tracker_rotate_prompt),
                modifier = Modifier.safeDrawingPadding()
            )
            state.isFinished -> Box(modifier = Modifier.fillMaxSize().safeDrawingPadding()) {
                GameSummary(state = state, onBack = onFinish)
            }
            state.players.isEmpty() -> Box(modifier = Modifier.fillMaxSize().safeDrawingPadding()) {
                LoadingTable(remoteSync = state.remoteSync, onBack = onFinish)
            }
            else -> {
                QuadrantGrid(
                    modifier = Modifier.safeDrawingPadding(),
                    players = state.players,
                    localSeatId = state.localSeatId,
                    expandedPlayerId = expandedPlayerId,
                    onToggleExpand = { id -> expandedPlayerId = if (expandedPlayerId == id) null else id },
                    onCommanderDamageChange = viewModel::adjustCommanderDamage,
                    onPoisonChange = viewModel::adjustPoison,
                    onPassTurn = { viewModel.nextTurn() },
                    onDamageDrop = { sourceId, targetId ->
                        damageSourceId = sourceId
                        damageTargetId = targetId
                    },
                    activeTurnPlayerId = state.currentTurnPlayerId,
                    turnTimeOf = viewModel::turnTimeOf,
                    clockRunningFor = state.currentTurnPlayerId.takeIf { state.turnClockRunningSince != null },
                    turnNumber = state.currentTurn,
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
                    // Anchored to the window's top edge, which is now behind the status bar.
                    modifier = Modifier
                        .align(Alignment.TopCenter)
                        .windowInsetsPadding(WindowInsets.safeDrawing.only(WindowInsetsSides.Top))
                        .padding(top = 4.dp)
                )

                PauseButton(
                    onClick = { paused = !paused },
                    modifier = Modifier.align(Alignment.Center)
                )

                if (showStarterBanner && state.startingPlayerId != null) {
                    val starterName = state.players.firstOrNull { it.id == state.startingPlayerId }?.name
                    StarterBanner(name = starterName ?: "", modifier = Modifier.matchParentSize())
                }

                val damageSource = state.players.firstOrNull { it.id == damageSourceId }
                val damageTarget = state.players.firstOrNull { it.id == damageTargetId }
                if (damageSource != null && damageTarget != null) {
                    val closeDamage = {
                        damageSourceId = null
                        damageTargetId = null
                    }
                    // Faces whoever made the drag: the attacker at a shared table, or this
                    // device's own seat in joined mode (where the drag ends on it).
                    val actorId = state.localSeatId ?: damageSource.id
                    DamageDialog(
                        source = damageSource,
                        target = damageTarget,
                        rotated = seatRows(state.players).first.any { it.id == actorId },
                        onApply = { amount, commander ->
                            if (damageSource.id == damageTarget.id) {
                                viewModel.gainLife(damageSource.id, amount)
                            } else {
                                viewModel.dealDamage(damageSource.id, damageTarget.id, amount, commander)
                            }
                            closeDamage()
                        },
                        onDismiss = closeDamage,
                        modifier = Modifier.matchParentSize()
                    )
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

/** Seat grid: first half "at the top of the table" (rotated 180°), the rest below. Works for 2-6. */
@Composable
private fun QuadrantGrid(
    modifier: Modifier = Modifier,
    players: List<PlayerState>,
    localSeatId: Int?,
    expandedPlayerId: Int?,
    onToggleExpand: (Int) -> Unit,
    onCommanderDamageChange: (targetPlayerId: Int, attackerId: Int, amount: Int) -> Unit,
    onPoisonChange: (playerId: Int, amount: Int) -> Unit,
    onPassTurn: () -> Unit,
    onDamageDrop: (sourceId: Int, targetId: Int) -> Unit,
    activeTurnPlayerId: Int?,
    turnTimeOf: (playerId: Int) -> Long,
    clockRunningFor: Int?,
    turnNumber: Int,
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

    // Where each seat sits inside the grid, so a drag can tell which seat the finger is over. The
    // bounds are read from the live coordinates at use time rather than cached, so they are never
    // stale after a rotation or a resize.
    val layout = remember { SeatLayoutCoordinates() }
    var damageDrag by remember { mutableStateOf<DamageDrag?>(null) }

    Box(
        modifier = modifier
            .fillMaxSize()
            .padding(4.dp)
            .onGloballyPositioned { layout.grid = it }
            .damageDragGesture(
                enabled = !randomizingStarter,
                players = players,
                localSeatId = localSeatId,
                seatAt = layout::seatAt,
                onDragChange = { damageDrag = it },
                onDrop = onDamageDrop
            )
    ) {
        Column(
            modifier = Modifier.fillMaxSize(),
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
                            onCommanderDamageChange = { attackerId, delta -> onCommanderDamageChange(player.id, attackerId, delta) },
                            onPoisonChange = { delta -> onPoisonChange(player.id, delta) },
                            onPassTurn = onPassTurn,
                            turnTime = { turnTimeOf(player.id) },
                            clockRunning = clockRunningFor == player.id,
                            turnNumber = turnNumber,
                            ring = ringFor(player.id),
                            rotated = rotated,
                            modifier = Modifier
                                .weight(1f)
                                .fillMaxHeight()
                                .onGloballyPositioned { layout.seats[player.id] = it }
                        )
                    }
                }
            }
        }

        damageDrag?.let { drag ->
            DamageDragOverlay(drag = drag, seatBounds = layout::boundsOf, modifier = Modifier.matchParentSize())
        }
    }
}

/**
 * The live layout coordinates of the seat grid and of every seat in it. Plain (non-state) fields:
 * they are only read inside the drag gesture and the overlay's draw pass, never to decide what to
 * compose.
 */
private class SeatLayoutCoordinates {
    var grid: LayoutCoordinates? = null
    val seats = mutableMapOf<Int, LayoutCoordinates>()

    /** [seatId]'s bounds in the grid's coordinates, or null if either isn't laid out. */
    fun boundsOf(seatId: Int): Rect? {
        val grid = grid?.takeIf { it.isAttached } ?: return null
        val seat = seats[seatId]?.takeIf { it.isAttached } ?: return null
        return grid.localBoundingBoxOf(seat)
    }

    /** The seat under [position], a point in the grid's coordinates. */
    fun seatAt(position: Offset): Int? = seats.keys.firstOrNull { boundsOf(it)?.contains(position) == true }
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
    val onLightSeat: Boolean
)

/**
 * Ink for a seat painted with commander art. The art's luminance is not the seat colour's, and it
 * varies across the crop, so the pair is not derived at all: the art always goes under a dark
 * scrim ([ArtScrim]) and the text on top is always white.
 */
private val ArtSeatInk = SeatInk(
    primary = Color.White,
    secondary = Color.White.copy(alpha = 0.72f),
    onLightSeat = false
)

/**
 * What keeps the life total readable over an arbitrary art crop. Heavier than the decorative
 * gradient a flat seat gets, and the reason [ArtSeatInk] can assume white text.
 */
private val ArtScrim = listOf(Color(0x990A0714), Color(0xCC0A0714))

private fun inkFor(seat: Color): SeatInk = if (seat.luminance() > 0.35f) {
    SeatInk(
        primary = Color.Black.copy(alpha = 0.85f),
        secondary = Color.Black.copy(alpha = 0.6f),
        onLightSeat = true
    )
} else {
    SeatInk(
        primary = Color.White,
        secondary = Color.White.copy(alpha = 0.72f),
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
    onCommanderDamageChange: (attackerId: Int, delta: Int) -> Unit,
    onPoisonChange: (Int) -> Unit,
    onPassTurn: () -> Unit,
    turnTime: () -> Long,
    clockRunning: Boolean,
    turnNumber: Int,
    ring: SeatRing?,
    rotated: Boolean,
    modifier: Modifier = Modifier
) {
    val eliminated = player.isEliminated()
    val deathAlpha by animateFloatAsState(targetValue = if (eliminated) 1f else 0f, animationSpec = tween(900), label = "death")
    val art = player.deckImageUrl
    val ink = if (art != null) ArtSeatInk else inkFor(player.color)
    val ringColor = when (ring) {
        SeatRing.Randomizing -> AppOnBackground
        SeatRing.ActiveTurn -> AccentSoft
        null -> null
    }
    // The art covers the seat colour, which is also how the commander-damage grids name their
    // opponents, so the colour comes back as the seat's outline whenever no ring is claiming it.
    val outlineColor = ringColor ?: player.color.takeIf { art != null }
    val outlineWidth = if (ringColor != null) 4.dp else 3.dp

    BoxWithConstraints(
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
            .then(if (outlineColor != null) Modifier.border(outlineWidth, outlineColor, QuadrantShape) else Modifier)
    ) {
        // A known player's seat wears their commander art; everyone else keeps the flat colour.
        // The seat colour stays underneath, so it is what shows while the image loads (or if it
        // never does).
        if (art != null) {
            AsyncImage(
                model = art,
                contentDescription = null,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize()
            )
        }

        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(
                    Brush.verticalGradient(
                        if (art != null) ArtScrim else listOf(Color(0x0D0A0714), Color(0x730A0714))
                    )
                )
        )

        // Every seat reserves the pass-turn bar's height, so its content area -- and the life total
        // centred in it -- sits at the same place on every seat whoever holds the turn.
        val contentHeight = maxHeight - PassTurnBarHeight
        val sizes = seatTextSizesFor(seatWidth = maxWidth, contentHeight = contentHeight)
        val damageTileSize = damageTileSizeFor(
            seatWidth = maxWidth,
            seatHeight = maxHeight,
            lifeSize = lifeSizeFor(maxWidth, contentHeight),
            // At least two digits, so the tiles don't grow when a total drops below 10.
            lifeDigits = player.life.toString().length.coerceAtLeast(2),
            // Every opponent could have dealt commander damage, plus the poison tile if any.
            tileCount = table.size - 1 + (if (player.poison > 0) 1 else 0)
        )

        // Life changes by dragging from one seat to another (see [damageDragGesture]), not by
        // tapping the seat. Not clickable itself, so a drag can start anywhere outside its buttons.
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(contentHeight)
                .padding(horizontal = SeatContentPadding, vertical = 10.dp)
        ) {
            Column(
                modifier = Modifier.align(Alignment.TopCenter),
                horizontalAlignment = Alignment.CenterHorizontally
            ) {
                Text(
                    player.name,
                    color = ink.primary.copy(alpha = 0.9f),
                    fontWeight = FontWeight.SemiBold,
                    fontSize = sizes.name,
                    maxLines = 1,
                    style = LocalTextStyle.current.copy(
                        shadow = SeatTextShadow.takeIf { !ink.onLightSeat }
                    )
                )
                if (player.mulligans > 0) {
                    Text(
                        stringResource(R.string.tracker_mulligans_suffix, player.mulligans),
                        color = ink.secondary,
                        fontSize = sizes.detail
                    )
                }
                TurnClock(
                    read = turnTime,
                    running = clockRunning,
                    turnNumber = turnNumber,
                    fontSize = sizes.detail,
                    ink = ink
                )
            }

            CommanderDamageTiles(
                player = player,
                table = table,
                tileSize = damageTileSize,
                onClick = onToggleExpand,
                modifier = Modifier.align(Alignment.BottomStart)
            )
        }

        // The life total is what the whole table reads, so it owns the true centre of the seat at a
        // size taken from the seat itself (it is sized off the content area, so it clears the
        // pass-turn bar). Tapping it opens the correction panel (commander damage and poison) even
        // before there is any tile to tap.
        Text(
            player.life.toString(),
            color = ink.primary,
            fontWeight = FontWeight.Bold,
            fontSize = sizes.life,
            maxLines = 1,
            textAlign = TextAlign.Center,
            modifier = Modifier
                .align(Alignment.Center)
                .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null, onClick = onToggleExpand),
            style = LocalTextStyle.current.copy(
                lineHeight = sizes.life,
                shadow = SeatTextShadow.takeIf { !ink.onLightSeat }
            )
        )

        // Only the turn owner gets the bar. Every seat reserves its height in the column above,
        // so the life total doesn't jump when the turn moves on.
        if (ring == SeatRing.ActiveTurn) {
            PassTurnBar(onClick = onPassTurn, modifier = Modifier.align(Alignment.BottomCenter))
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

/**
 * The seat's time on its own turns (see [GameViewModel.turnTimeOf]), shown from the start of its
 * first turn. Only the turn owner's clock ticks, so only that seat re-reads it every second; the
 * others show their banked total, which changes only when a turn ends. The turn owner's line also
 * carries the table's [turnNumber] -- it belongs to the whole table, so it sits with whoever is
 * playing it rather than being repeated on every seat.
 */
@Composable
private fun TurnClock(read: () -> Long, running: Boolean, turnNumber: Int, fontSize: TextUnit, ink: SeatInk) {
    val elapsedMs = if (running) {
        produceState(initialValue = read(), running) {
            while (true) {
                value = read()
                delay(TURN_CLOCK_TICK_MS)
            }
        }.value
    } else {
        read()
    }
    // Before the seat's first turn the line is still laid out, just invisible and silent, so the
    // life total below doesn't shift when the clock first appears.
    val visible = running || elapsedMs > 0L
    val time = formatTurnClock(elapsedMs)

    Text(
        if (running) "${stringResource(R.string.tracker_turn_label, turnNumber)} · $time" else time,
        color = if (running) ink.primary else ink.secondary,
        fontWeight = if (running) FontWeight.Bold else FontWeight.Normal,
        fontSize = fontSize,
        style = LocalTextStyle.current.copy(shadow = SeatTextShadow.takeIf { !ink.onLightSeat }),
        modifier = if (visible) Modifier else Modifier.alpha(0f).clearAndSetSemantics {}
    )
}

/** Re-reads the running clock more than once a second, so the seconds never visibly skip. */
private const val TURN_CLOCK_TICK_MS = 250L

/** `m:ss`, or `h:mm:ss` once a seat has spent an hour on its turns. */
internal fun formatTurnClock(ms: Long): String {
    val totalSeconds = ms / 1000
    val hours = totalSeconds / 3600
    val minutes = totalSeconds / 60 % 60
    val seconds = totalSeconds % 60
    // Locale.ROOT: plain ASCII digits whatever the device language.
    return if (hours > 0) {
        String.format(Locale.ROOT, "%d:%02d:%02d", hours, minutes, seconds)
    } else {
        String.format(Locale.ROOT, "%d:%02d", minutes, seconds)
    }
}

/**
 * Full-width bar along the seat's own bottom edge (the top of the screen for a rotated seat). It
 * wears the accent gradient of the app's primary buttons, matching the active seat's violet ring;
 * the quadrant's clip rounds its outer corners.
 */
@Composable
private fun PassTurnBar(onClick: () -> Unit, modifier: Modifier = Modifier) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .height(PassTurnBarHeight)
            .background(AccentGradient)
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Text(
            stringResource(R.string.tracker_pass_turn).uppercase(),
            color = AppBackgroundDeep,
            fontWeight = FontWeight.Bold,
            fontSize = 14.sp,
            letterSpacing = 1.sp
        )
    }
}

/**
 * The commander damage this seat has taken, one tile per opponent who has dealt any: the tile
 * wears that opponent's commander art (their seat colour when there is none) with the damage on
 * top, so a glance says whose commander is closing in. Poison gets a tile of its own. The row
 * always reserves its height, so the life total doesn't move when the first tile appears. Tapping
 * a tile opens [CommanderDamagePanel] to correct a value.
 */
@Composable
private fun CommanderDamageTiles(
    player: PlayerState,
    table: List<PlayerState>,
    tileSize: Dp,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val attackers = table.filter { it.id != player.id && (player.commanderDamage[it.id] ?: 0) > 0 }

    Row(
        modifier = modifier.height(tileSize),
        horizontalArrangement = Arrangement.spacedBy(DamageTileGap)
    ) {
        attackers.forEach { attacker ->
            val damage = player.commanderDamage.getValue(attacker.id)
            DamageTile(
                artUrl = attacker.deckImageUrl,
                fallbackColor = attacker.color,
                value = damage,
                valueColor = if (damage >= COMMANDER_DAMAGE_LETHAL) StatusDanger else Color.White,
                size = tileSize,
                onClick = onClick
            )
        }
        if (player.poison > 0) {
            DamageTile(
                artUrl = null,
                fallbackColor = Color(0xFF14331F),
                value = player.poison,
                valueColor = StatusPoison,
                size = tileSize,
                label = "☠",
                onClick = onClick
            )
        }
    }
}

/** Font sizes for one seat, derived from its size rather than fixed -- see [seatTextSizesFor]. */
private data class SeatTextSizes(val life: TextUnit, val name: TextUnit, val detail: TextUnit)

/**
 * Sizes a seat's text from the seat itself, so it fills a tablet seat and still fits a phone one:
 *  - the life total takes 45% of the content height, capped at 30% of the width so a three-digit
 *    total still fits across;
 *  - the name and the clock line scale more gently, within readable bounds.
 * Worked out in dp and converted with [Density.toSp], which divides out the system font scale:
 * these are proportions of the seat, and a larger font setting would otherwise push the life
 * total out of it.
 */
@Composable
private fun seatTextSizesFor(seatWidth: Dp, contentHeight: Dp): SeatTextSizes {
    val density = LocalDensity.current
    val life = lifeSizeFor(seatWidth, contentHeight)
    val name = (contentHeight * 0.07f).coerceIn(12.dp, 28.dp)
    val detail = name * 0.8f
    return with(density) { SeatTextSizes(life = life.toSp(), name = name.toSp(), detail = detail.toSp()) }
}

/** The life total's font size, in dp -- see [seatTextSizesFor]. */
private fun lifeSizeFor(seatWidth: Dp, contentHeight: Dp): Dp = minOf(contentHeight * 0.45f, seatWidth * 0.30f)

/** Smallest tile: still a comfortable touch target on a phone held sideways. */
private val MinDamageTileSize = 44.dp
private val DamageTileGap = 6.dp

/** Horizontal padding of a seat's content, which the tiles row starts inside of. */
private val SeatContentPadding = 12.dp

/**
 * Rough advance of one digit of the (bold) life total, as a share of its font size -- generous,
 * so the estimate errs towards leaving a gap rather than an overlap.
 */
private const val LIFE_DIGIT_WIDTH = 0.62f

/**
 * Tiles grow with the seat, so they read from across the table on a tablet (12% of the seat's
 * width, at most 22% of its height), but they must also fit, side by side, in the room left of the
 * centred life total: with a full table's worth of them ([tileCount]) the row still stops short of
 * the number. Sized for the most tiles the seat can show rather than the ones showing now, so a new
 * tile doesn't shrink the others. Never under [MinDamageTileSize].
 */
private fun damageTileSizeFor(seatWidth: Dp, seatHeight: Dp, lifeSize: Dp, lifeDigits: Int, tileCount: Int): Dp {
    val preferred = minOf(seatWidth * 0.12f, seatHeight * 0.22f)
    val lifeWidth = lifeSize * (LIFE_DIGIT_WIDTH * lifeDigits)
    val room = (seatWidth - lifeWidth) / 2 - SeatContentPadding - DamageTileGap
    val count = tileCount.coerceAtLeast(1)
    val fitting = (room - DamageTileGap * (count - 1)) / count
    return minOf(preferred, fitting).coerceAtLeast(MinDamageTileSize)
}

/** One tile of [CommanderDamageTiles]: art (or a flat colour) under a scrim, a big number on top. */
@Composable
private fun DamageTile(
    artUrl: String?,
    fallbackColor: Color,
    value: Int,
    valueColor: Color,
    size: Dp,
    onClick: () -> Unit,
    label: String? = null
) {
    val shape = RoundedCornerShape(size * 0.2f)
    Box(
        modifier = Modifier
            .size(size)
            .clip(shape)
            .background(fallbackColor)
            .border(1.5.dp, Color.Black.copy(alpha = 0.6f), shape)
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        if (artUrl != null) {
            AsyncImage(
                model = artUrl,
                contentDescription = null,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize()
            )
        }
        Box(modifier = Modifier.fillMaxSize().background(Color.Black.copy(alpha = 0.35f)))
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            // The number scales with the tile: 22sp on the smallest one.
            if (label != null) Text(label, color = valueColor, fontSize = (size.value * 0.2f).sp)
            Text(
                value.toString(),
                color = valueColor,
                fontWeight = FontWeight.Bold,
                fontSize = (size.value * if (label != null) 0.3f else 0.42f).sp,
                style = LocalTextStyle.current.copy(shadow = SeatTextShadow)
            )
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
            text = remoteSync.failure?.message() ?: stringResource(R.string.tracker_loading_joined_game),
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
        RemoteSyncStatus.Disabled -> stringResource(R.string.tracker_sync_local_only)
        RemoteSyncStatus.WaitingForPlayers -> stringResource(R.string.tracker_sync_waiting_players)
        RemoteSyncStatus.Failed ->
            remoteSync.failure?.message() ?: stringResource(R.string.error_api_unexpected)
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
