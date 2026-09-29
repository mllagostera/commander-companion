package com.commandercompanion.presentation.screens.game

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.gestures.awaitTouchSlopOrCancellation
import androidx.compose.foundation.gestures.drag
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.commandercompanion.R
import com.commandercompanion.presentation.components.GradientButton
import com.commandercompanion.presentation.components.GradientOutlineButton
import com.commandercompanion.presentation.components.PillSegmentedControl
import com.commandercompanion.presentation.theme.AccentSoft
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppOnBackground
import com.commandercompanion.presentation.theme.AppOutline
import com.commandercompanion.presentation.theme.AppSurface
import com.commandercompanion.presentation.theme.StatusDanger
import com.commandercompanion.presentation.theme.StatusSuccess
import kotlin.math.atan2
import kotlin.math.cos
import kotlin.math.sin

/**
 * A damage drag in progress: the finger went down on [sourceId]'s seat at [start] and is now at
 * [current], over [targetId] if that seat can take the damage. Positions are in the coordinates of
 * the seat grid, the same ones the seat bounds are measured in.
 */
data class DamageDrag(
    val sourceId: Int,
    val start: Offset,
    val current: Offset,
    val targetId: Int?
)

/** A drag that ends on the seat it started from: that player gains life instead of dealing damage. */
internal val DamageDrag.isLifeGain: Boolean get() = targetId == sourceId

/**
 * Where a drag from [source] may end. On another live seat it deals damage; back on [source]
 * itself it gains life. Pass-and-play ([localSeatId] null) allows it for every live seat. Joined
 * mode only records changes to this device's own seat (see [GameState.localSeatId]), so there the
 * drag must END on the local seat: the player records the damage they took, from whoever dealt it,
 * or the life they gained.
 */
internal fun canDropOn(source: PlayerState, target: PlayerState, localSeatId: Int?): Boolean =
    !source.isEliminated() &&
        !target.isEliminated() &&
        (localSeatId == null || target.id == localSeatId)

/**
 * Tracks a drag across the whole seat grid. [seatAt] maps a grid position to the seat under it.
 * Taps are left alone -- the gesture only claims the pointer once it has moved past the touch
 * slop, so a seat's buttons (pass turn, the commander-damage tiles) keep working as before.
 *
 * The source is the seat under the finger when it first touched down. `detectDragGestures` only
 * reports where the finger was once it crossed the slop, which a fast flick can place in the
 * neighbouring seat -- turning damage to someone else into life gained by them.
 */
internal fun Modifier.damageDragGesture(
    enabled: Boolean,
    players: List<PlayerState>,
    localSeatId: Int?,
    seatAt: (Offset) -> Int?,
    onDragChange: (DamageDrag?) -> Unit,
    onDrop: (sourceId: Int, targetId: Int) -> Unit
): Modifier = if (!enabled) this else pointerInput(players, localSeatId) {
    val byId = players.associateBy { it.id }
    awaitEachGesture {
        val down = awaitFirstDown(requireUnconsumed = false)
        val source = seatAt(down.position)?.let(byId::get)?.takeIf { !it.isEliminated() }
            ?: return@awaitEachGesture
        val slopChange = awaitTouchSlopOrCancellation(down.id) { change, _ -> change.consume() }
            ?: return@awaitEachGesture

        fun update(drag: DamageDrag, position: Offset): DamageDrag {
            val target = seatAt(position)?.let(byId::get)?.takeIf { canDropOn(source, it, localSeatId) }
            return drag.copy(current = position, targetId = target?.id)
        }

        var dragState = update(DamageDrag(source.id, down.position, down.position, null), slopChange.position)
        onDragChange(dragState)
        val completed = drag(slopChange.id) { change ->
            change.consume()
            dragState = update(dragState, change.position)
            onDragChange(dragState)
        }
        if (completed) dragState.targetId?.let { onDrop(source.id, it) }
        onDragChange(null)
    }
}

/**
 * The arrow from the source seat to the finger, over a dimmed source seat and a lit target seat.
 * Drawn on top of the grid and never hit-tested, so it can't steal the drag it is drawing.
 */
@Composable
internal fun DamageDragOverlay(drag: DamageDrag, seatBounds: (Int) -> Rect?, modifier: Modifier = Modifier) {
    // Red for damage to someone else, green for life gained back on the same seat.
    val dropColor = if (drag.isLifeGain) StatusSuccess else StatusDanger
    Canvas(modifier = modifier) {
        val corner = CornerRadius(22.dp.toPx())
        if (!drag.isLifeGain) {
            seatBounds(drag.sourceId)?.let { bounds ->
                drawRoundRect(AccentSoft.copy(alpha = 0.16f), bounds.topLeft, bounds.size, corner)
            }
        }
        drag.targetId?.let(seatBounds)?.let { bounds ->
            drawRoundRect(dropColor.copy(alpha = 0.22f), bounds.topLeft, bounds.size, corner)
            drawRoundRect(dropColor, bounds.topLeft, bounds.size, corner, style = Stroke(4.dp.toPx()))
        }

        val color = if (drag.targetId != null) dropColor else AccentSoft
        val width = 6.dp.toPx()
        drawCircle(color, radius = 9.dp.toPx(), center = drag.start)
        drawLine(color, drag.start, drag.current, strokeWidth = width, cap = StrokeCap.Round)

        // Arrowhead at the finger, pointing along the drag.
        val angle = atan2(drag.current.y - drag.start.y, drag.current.x - drag.start.x)
        val head = 26.dp.toPx()
        listOf(angle + ARROW_SPREAD, angle - ARROW_SPREAD).forEach { side ->
            val tip = drag.current - Offset(cos(side) * head, sin(side) * head)
            drawLine(color, drag.current, tip, strokeWidth = width, cap = StrokeCap.Round)
        }
    }
}

private const val ARROW_SPREAD = 0.5f

/** Amounts a single drag can assign; commander damage is lethal well before the top. */
private const val MAX_AMOUNT = 99

/**
 * Picks how much damage [source] deals to [target] and whether it is commander damage, with a
 * preview of what it does to the target before anything is applied. When [source] and [target]
 * are the same seat it picks how much life that player gains instead, with no damage type.
 * [rotated] turns the dialog to face a player sitting on the far side of the table, like their seat.
 */
@Composable
internal fun DamageDialog(
    source: PlayerState,
    target: PlayerState,
    rotated: Boolean,
    onApply: (amount: Int, commander: Boolean) -> Unit,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier
) {
    var amount by rememberSaveable { mutableIntStateOf(1) }
    var commander by rememberSaveable { mutableStateOf(false) }
    val lifeGain = source.id == target.id
    val currentCommanderDamage = target.commanderDamage[source.id] ?: 0
    // Back closes the dialog, not the game underneath it.
    BackHandler(onBack = onDismiss)

    Box(
        modifier = modifier
            .background(DamageScrim)
            .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null, onClick = onDismiss),
        contentAlignment = Alignment.Center
    ) {
        Column(
            modifier = Modifier
                .then(if (rotated) Modifier.rotate(180f) else Modifier)
                .padding(16.dp)
                .widthIn(max = 420.dp)
                .clip(RoundedCornerShape(24.dp))
                .background(AppSurface)
                .border(1.dp, AppOutline, RoundedCornerShape(24.dp))
                // Swallows taps on the card so only the scrim around it dismisses.
                .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null) {}
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 16.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Text(
                stringResource(if (lifeGain) R.string.tracker_life_gain_title else R.string.tracker_damage_title).uppercase(),
                color = if (lifeGain) StatusSuccess else AccentSoft,
                fontWeight = FontWeight.SemiBold,
                fontSize = 11.sp,
                letterSpacing = 1.sp
            )
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                SeatTag(source)
                if (!lifeGain) {
                    Text("→", color = AppFaint, fontSize = 18.sp)
                    SeatTag(target)
                }
            }

            if (!lifeGain) {
                PillSegmentedControl(
                    options = listOf(
                        false to stringResource(R.string.tracker_damage_normal),
                        true to stringResource(R.string.tracker_damage_commander)
                    ),
                    selected = commander,
                    onSelected = { commander = it }
                )
            }

            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(20.dp)) {
                AmountStepButton(
                    symbol = "−",
                    label = stringResource(R.string.tracker_amount_decrease),
                    enabled = amount > 1,
                    onClick = { amount = (amount - 1).coerceAtLeast(1) }
                )
                Text(
                    amount.toString(),
                    color = AppOnBackground,
                    fontWeight = FontWeight.Bold,
                    fontSize = 48.sp,
                    textAlign = TextAlign.Center,
                    modifier = Modifier.widthIn(min = 80.dp)
                )
                AmountStepButton(
                    symbol = "+",
                    label = stringResource(R.string.tracker_amount_increase),
                    enabled = amount < MAX_AMOUNT,
                    onClick = { amount = (amount + 1).coerceAtMost(MAX_AMOUNT) }
                )
            }

            Text(
                stringResource(
                    R.string.tracker_damage_life_preview,
                    target.life,
                    if (lifeGain) target.life + amount else target.life - amount
                ),
                color = AppOnBackground,
                fontSize = 13.sp
            )
            if (commander && !lifeGain) {
                val after = currentCommanderDamage + amount
                Text(
                    stringResource(
                        R.string.tracker_damage_commander_preview,
                        currentCommanderDamage,
                        after,
                        COMMANDER_DAMAGE_LETHAL
                    ),
                    color = if (after >= COMMANDER_DAMAGE_LETHAL) StatusDanger else AppOnBackground,
                    fontSize = 13.sp
                )
            }

            Row(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.fillMaxWidth()) {
                GradientOutlineButton(
                    text = stringResource(R.string.tracker_damage_cancel),
                    onClick = onDismiss,
                    modifier = Modifier.weight(1f)
                )
                GradientButton(
                    text = stringResource(R.string.tracker_damage_apply),
                    onClick = { onApply(amount, commander && !lifeGain) },
                    modifier = Modifier.weight(1f)
                )
            }
        }
    }
}

private val DamageScrim = Color(0xD9050308)

/** A seat's name next to its colour dot, the way the commander-damage grids name seats. */
@Composable
private fun SeatTag(player: PlayerState) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        Box(modifier = Modifier.size(12.dp).clip(CircleShape).background(player.color))
        Text(player.name, color = AppOnBackground, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
    }
}

@Composable
private fun AmountStepButton(symbol: String, label: String, enabled: Boolean, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .size(52.dp)
            .clip(CircleShape)
            .background(Color.White.copy(alpha = if (enabled) 0.08f else 0.03f))
            .border(1.dp, AppOutline, CircleShape)
            .clickable(enabled = enabled, onClick = onClick)
            .semantics { contentDescription = label },
        contentAlignment = Alignment.Center
    ) {
        Text(symbol, color = if (enabled) AppOnBackground else AppFaint, fontSize = 26.sp, fontWeight = FontWeight.Bold)
    }
}
