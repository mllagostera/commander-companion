package com.vansid.tapeandocartones.presentation.screens.pregame

import android.content.res.Configuration
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.listSaver
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.snapshots.SnapshotStateList
import androidx.compose.runtime.toMutableStateList
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import coil3.compose.AsyncImage
import com.vansid.tapeandocartones.R
import com.vansid.tapeandocartones.domain.model.Deck
import com.vansid.tapeandocartones.domain.model.PlaygroupMember
import com.vansid.tapeandocartones.presentation.components.DeckArtChip
import com.vansid.tapeandocartones.presentation.components.KeepScreenOn
import com.vansid.tapeandocartones.presentation.components.RotateDevicePrompt
import com.vansid.tapeandocartones.presentation.components.SelectableChip
import com.vansid.tapeandocartones.presentation.navigation.PlayerConfig
import com.vansid.tapeandocartones.presentation.navigation.decodePlayerConfigs
import com.vansid.tapeandocartones.presentation.navigation.encodePlayerConfigs
import com.vansid.tapeandocartones.presentation.theme.AccentGradient
import com.vansid.tapeandocartones.presentation.theme.AccentSoft
import com.vansid.tapeandocartones.presentation.theme.AppBackgroundDeep
import com.vansid.tapeandocartones.presentation.theme.AppFaint
import com.vansid.tapeandocartones.presentation.theme.AppMuted
import com.vansid.tapeandocartones.presentation.theme.AppOnBackground
import com.vansid.tapeandocartones.presentation.theme.colorForKey
import kotlin.random.Random

private fun <T> stateListSaver() = listSaver<SnapshotStateList<T>, T>(
    save = { it.toList() },
    restore = { it.toMutableStateList() }
)

/**
 * What a seat is asking for right now. A Group-mode seat walks through them in the order things
 * happen at a real table — sit down, pick a deck, draw and decide on mulligans — and only shows
 * the current one. Guests and members without decks skip straight to [Mulligans]; Casual mode
 * already has its seats from Setup, so it only ever shows [Mulligans].
 */
private enum class SeatStep { Member, Deck, Mulligans }

/**
 * Only makes sense played in landscape (each seat faces its own edge of the
 * "table"), so in portrait we show the prompt to rotate the device — real orientation
 * via `LocalConfiguration`, not a simulated timer.
 *
 * In Group mode ([playgroupId] non-null), this is also where seats get assigned to
 * playgroup members and decks — [PlayerSetupScreen][com.vansid.tapeandocartones.presentation.screens.setup.PlayerSetupScreen]
 * only picks the playgroup and player count, matching the mockup ("Asiento, color y
 * deck se eligen al empezar."), and each seat moves through [SeatStep]. Casual mode
 * ([playgroupId] null) keeps the seat/color already chosen in Setup and only adds
 * mulligans + the starting-player draw here.
 */
@Composable
fun PreGameScreen(
    playersEncoded: String,
    playgroupId: String?,
    onContinue: (playersEncoded: String, startingPlayerSeat: Int) -> Unit,
    viewModel: PreGameViewModel = hiltViewModel()
) {
    // Already at the table (starter draw, mulligans): same as the tracker, the screen must not time out.
    KeepScreenOn()
    val configs = remember { decodePlayerConfigs(playersEncoded) }
    val mulligans = rememberSaveable(saver = stateListSaver<Int>()) {
        mutableStateListOf(*Array(configs.size) { 0 })
    }

    // Group mode only: which member (or "" for a Guest) sits at each seat, and their deck.
    // A null member means the seat hasn't been assigned yet — the member picker keeps showing.
    // A null deck means it hasn't been decided yet; "" means the member sits down without one.
    val seatMemberUserIds = rememberSaveable(saver = stateListSaver<String?>()) {
        mutableStateListOf(*arrayOfNulls<String>(configs.size))
    }
    val seatMemberUsernames = rememberSaveable(saver = stateListSaver<String?>()) {
        mutableStateListOf(*arrayOfNulls<String>(configs.size))
    }
    val seatDeckIds = rememberSaveable(saver = stateListSaver<String?>()) {
        mutableStateListOf(*arrayOfNulls<String>(configs.size))
    }
    // Back on the deck step to change an already chosen deck, which stays chosen meanwhile.
    val seatChangingDeck = rememberSaveable(saver = stateListSaver<Boolean>()) {
        mutableStateListOf(*Array(configs.size) { false })
    }

    LaunchedEffect(playgroupId) {
        if (playgroupId != null) viewModel.loadPlaygroup(playgroupId)
    }

    // Preselect the deck each member last played here: repeating a deck is the common case, so it
    // takes the seat straight to mulligans. Only fills an undecided seat, so it never overrides a
    // pick — and changing deck keeps the old one chosen, so this doesn't re-fire then either.
    seatMemberUserIds.forEachIndexed { index, userId ->
        key(index) {
            val suggested = userId?.takeIf { it.isNotEmpty() }?.let(viewModel::lastDeckFor)
            LaunchedEffect(userId, suggested) {
                if (suggested != null && seatDeckIds[index] == null) seatDeckIds[index] = suggested.id
            }
        }
    }

    val guestLabel = stringResource(R.string.setup_guest)
    // Group mode: the seat's assigned name once decided, falling back to the Setup
    // placeholder ("Jugador N") while still unassigned — used for both the starter draw
    // banner and the final merge into PlayerConfig, so both agree once a seat locks in.
    fun displayName(index: Int): String = when {
        playgroupId == null -> configs[index].name
        seatMemberUsernames[index] != null -> seatMemberUsernames[index]!!
        seatMemberUserIds[index] != null -> "$guestLabel ${index + 1}"
        else -> configs[index].name
    }

    fun seatState(index: Int): SeatState {
        val userId = seatMemberUserIds[index]
        val decks = userId?.takeIf { it.isNotEmpty() }?.let(viewModel::decksFor).orEmpty()
        val deckId = seatDeckIds[index]
        val step = when {
            playgroupId == null -> SeatStep.Mulligans
            userId == null -> SeatStep.Member
            userId.isEmpty() -> SeatStep.Mulligans
            !viewModel.decksLoaded(userId) -> SeatStep.Deck
            decks.isEmpty() -> SeatStep.Mulligans
            deckId == null || seatChangingDeck[index] -> SeatStep.Deck
            else -> SeatStep.Mulligans
        }
        return SeatState(
            isGroup = playgroupId != null,
            step = step,
            assignedUsername = seatMemberUsernames[index],
            memberDecks = decks,
            decksLoading = userId != null && userId.isNotEmpty() && !viewModel.decksLoaded(userId),
            deckId = deckId,
            mulligans = mulligans[index]
        )
    }

    val isLandscape = LocalConfiguration.current.orientation == Configuration.ORIENTATION_LANDSCAPE

    // Edge-to-edge: the deep violet fills the whole window, and the safe-area padding goes on the
    // children that are touched rather than on this Box, so the seat grid stays clear of the
    // landscape gesture bar and any display cutout. The play button is centred, so it needs none.
    Box(modifier = Modifier.fillMaxSize().background(AppBackgroundDeep)) {
        if (!isLandscape) {
            RotateDevicePrompt(
                message = stringResource(R.string.pregame_rotate_prompt),
                modifier = Modifier.safeDrawingPadding()
            )
        } else {
            val availableMembers = viewModel.playgroup?.members.orEmpty()
            SeatGrid(
                modifier = Modifier.safeDrawingPadding(),
                seatCount = configs.size
            ) { index, rotated, modifier ->
                SeatCard(
                    seatIndex = index,
                    config = configs[index],
                    state = seatState(index),
                    rotated = rotated,
                    availableMembers = availableMembers.filter { it.userId !in seatMemberUserIds.filterIndexed { i, _ -> i != index } },
                    ownUsername = viewModel.ownUsername,
                    actions = SeatActions(
                        onMemberSelected = { member ->
                            seatMemberUserIds[index] = member?.userId ?: ""
                            seatMemberUsernames[index] = member?.username
                            if (member != null && playgroupId != null) {
                                viewModel.loadMemberDecks(playgroupId, member.userId)
                            }
                        },
                        onFreeSeat = {
                            seatMemberUserIds[index] = null
                            seatMemberUsernames[index] = null
                            seatDeckIds[index] = null
                            seatChangingDeck[index] = false
                            mulligans[index] = 0
                        },
                        onDeckSelected = { deckId ->
                            seatDeckIds[index] = deckId
                            seatChangingDeck[index] = false
                        },
                        onChangeDeck = { seatChangingDeck[index] = true },
                        onIncrement = { mulligans[index] = mulligans[index] + 1 },
                        onDecrement = { mulligans[index] = (mulligans[index] - 1).coerceAtLeast(0) }
                    ),
                    modifier = modifier
                )
            }

            PlayButton(
                onClick = {
                    val updatedConfigs = configs.mapIndexed { index, config ->
                        if (playgroupId != null) {
                            val userId = seatMemberUserIds[index]?.takeIf { it.isNotEmpty() }
                            val deckId = userId?.let { seatDeckIds[index] }?.takeIf { it.isNotEmpty() }
                            // The tracker paints the seat with this art; the deck is already
                            // loaded here, so it travels with the seat instead of being fetched
                            // again once the game is on screen.
                            val deckImageUrl = userId
                                ?.let { id -> viewModel.decksFor(id).firstOrNull { deck -> deck.id == deckId } }
                                ?.imageUrl
                            config.copy(
                                name = displayName(index),
                                mulligans = mulligans[index],
                                assignedUserId = userId,
                                assignedUsername = seatMemberUsernames[index],
                                deckId = deckId,
                                deckImageUrl = deckImageUrl
                            )
                        } else {
                            config.copy(mulligans = mulligans[index])
                        }
                    }
                    if (playgroupId != null) {
                        viewModel.rememberDecks(
                            playgroupId,
                            updatedConfigs.mapNotNull { config ->
                                val userId = config.assignedUserId ?: return@mapNotNull null
                                val deckId = config.deckId ?: return@mapNotNull null
                                userId to deckId
                            }.toMap()
                        )
                    }
                    onContinue(encodePlayerConfigs(updatedConfigs), Random.nextInt(configs.size))
                },
                modifier = Modifier.align(Alignment.Center)
            )
        }
    }
}

/** Everything a [SeatCard] renders, derived from the screen's per-seat state. */
private data class SeatState(
    val isGroup: Boolean,
    val step: SeatStep,
    val assignedUsername: String?,
    val memberDecks: List<Deck>,
    val decksLoading: Boolean,
    /** null: not decided yet; "": the member sits down without a deck. */
    val deckId: String?,
    val mulligans: Int
) {
    val selectedDeck: Deck? get() = memberDecks.firstOrNull { it.id == deckId }
}

private class SeatActions(
    val onMemberSelected: (PlaygroupMember?) -> Unit,
    val onFreeSeat: () -> Unit,
    val onDeckSelected: (deckId: String) -> Unit,
    val onChangeDeck: () -> Unit,
    val onIncrement: () -> Unit,
    val onDecrement: () -> Unit
)

/** Lays the seats out the way the table is seated: the first half on top (rotated 180°), the rest below. */
@Composable
private fun SeatGrid(
    modifier: Modifier = Modifier,
    seatCount: Int,
    seat: @Composable (index: Int, rotated: Boolean, modifier: Modifier) -> Unit
) {
    val topCount = (seatCount + 1) / 2
    Column(
        modifier = modifier.fillMaxSize().padding(4.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp)
    ) {
        listOf(0 until topCount to true, topCount until seatCount to false).forEach { (indices, rotated) ->
            Row(
                modifier = Modifier.weight(1f).fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                indices.forEach { index -> seat(index, rotated, Modifier.weight(1f).fillMaxHeight()) }
            }
        }
    }
}

@Composable
private fun SeatCard(
    seatIndex: Int,
    config: PlayerConfig,
    state: SeatState,
    rotated: Boolean,
    availableMembers: List<PlaygroupMember>,
    ownUsername: String?,
    actions: SeatActions,
    modifier: Modifier = Modifier
) {
    Box(
        modifier = modifier
            .then(if (rotated) Modifier.rotate(180f) else Modifier)
            .clip(RoundedCornerShape(22.dp))
            .background(Color.White.copy(alpha = 0.04f))
    ) {
        // Once the deck is locked in, the seat already wears its art — a preview of the tracker.
        val artUrl = state.selectedDeck?.imageUrl.takeIf { state.step == SeatStep.Mulligans }
        if (artUrl != null) {
            AsyncImage(
                model = artUrl,
                contentDescription = null,
                contentScale = ContentScale.Crop,
                alpha = 0.45f,
                modifier = Modifier.fillMaxSize()
            )
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(Brush.verticalGradient(listOf(AppBackgroundDeep.copy(alpha = 0.3f), AppBackgroundDeep.copy(alpha = 0.85f))))
            )
        }

        Column(
            modifier = Modifier.fillMaxSize().padding(8.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            Text(stringResource(R.string.pregame_seat_label, seatIndex + 1), fontSize = 10.sp, color = AppFaint)
            Spacer(modifier = Modifier.height(6.dp))

            AnimatedContent(
                targetState = state.step,
                transitionSpec = { fadeIn() togetherWith fadeOut() },
                contentAlignment = Alignment.Center,
                label = "seatStep"
            ) { step ->
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    when (step) {
                        SeatStep.Member -> SeatMemberPicker(
                            availableMembers = availableMembers,
                            ownUsername = ownUsername,
                            onMemberSelected = actions.onMemberSelected
                        )
                        SeatStep.Deck -> SeatDeckPicker(state = state, actions = actions)
                        SeatStep.Mulligans -> SeatMulligans(
                            config = config,
                            state = state,
                            actions = actions
                        )
                    }
                }
            }
        }
    }
}

/** Step 1 — pick a Guest or an available playgroup member. */
@Composable
private fun SeatMemberPicker(
    availableMembers: List<PlaygroupMember>,
    ownUsername: String?,
    onMemberSelected: (PlaygroupMember?) -> Unit
) {
    val youSuffix = stringResource(R.string.common_you_suffix)
    FlowRow(
        horizontalArrangement = Arrangement.spacedBy(4.dp, Alignment.CenterHorizontally),
        verticalArrangement = Arrangement.spacedBy(4.dp),
        modifier = Modifier.fillMaxWidth()
    ) {
        SelectableChip(
            label = stringResource(R.string.setup_guest),
            selected = false,
            onClick = { onMemberSelected(null) }
        )
        availableMembers.forEach { member ->
            val label = if (member.username == ownUsername) "${member.username} $youSuffix" else member.username
            SelectableChip(label = label, selected = false, onClick = { onMemberSelected(member) })
        }
    }
}

/**
 * Step 2 — the member's decks plus "no deck". When changing an already chosen deck, it stays
 * highlighted and scrolled into view, so switching is a single tap.
 */
@Composable
private fun SeatDeckPicker(state: SeatState, actions: SeatActions) {
    SeatHeader(name = state.assignedUsername.orEmpty(), onFreeSeat = actions.onFreeSeat)
    Spacer(modifier = Modifier.height(8.dp))

    if (state.decksLoading) {
        CircularProgressIndicator(modifier = Modifier.size(20.dp), strokeWidth = 2.dp, color = AccentSoft)
        return
    }

    val selectedIndex = state.memberDecks.indexOfFirst { it.id == state.deckId }
    val listState = rememberLazyListState(initialFirstVisibleItemIndex = selectedIndex.coerceAtLeast(0))
    LazyRow(
        state = listState,
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        contentPadding = PaddingValues(horizontal = 4.dp)
    ) {
        items(state.memberDecks, key = { it.id }) { deck ->
            DeckArtChip(
                name = deck.name,
                imageUrl = deck.imageUrl,
                selected = deck.id == state.deckId,
                onClick = { actions.onDeckSelected(deck.id) },
                width = 76.dp,
                height = 52.dp,
                nameMaxLines = 2
            )
        }
        item(key = "no-deck") {
            DeckArtChip(
                name = stringResource(R.string.pregame_no_deck),
                imageUrl = null,
                selected = state.deckId == "",
                onClick = { actions.onDeckSelected("") },
                width = 76.dp,
                height = 52.dp
            )
        }
    }
}

/**
 * Step 3 — who sits here and with what (each tappable to go back), and a mulligan counter big
 * enough to use across the table. Mulligans are only recorded here and shown in the game's end
 * summary, never on the tracker.
 */
@Composable
private fun SeatMulligans(config: PlayerConfig, state: SeatState, actions: SeatActions) {
    if (state.isGroup) {
        SeatHeader(
            name = state.assignedUsername ?: stringResource(R.string.setup_guest),
            onFreeSeat = actions.onFreeSeat
        ) {
            // Guests have no account to attach a deck to, so there's nothing to change.
            if (state.assignedUsername != null && state.memberDecks.isNotEmpty()) {
                Text(" · ", fontSize = 13.sp, color = AppFaint)
                Text(
                    text = "${state.selectedDeck?.name ?: stringResource(R.string.pregame_no_deck)}  ✎",
                    fontSize = 12.sp,
                    color = AccentSoft,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier
                        .weight(1f, fill = false)
                        .clip(RoundedCornerShape(8.dp))
                        .clickable(onClick = actions.onChangeDeck)
                        .padding(horizontal = 4.dp, vertical = 2.dp)
                )
            }
        }
        if (state.assignedUsername != null && state.memberDecks.isEmpty()) {
            Text(
                stringResource(R.string.setup_member_no_decks, state.assignedUsername),
                color = AppFaint,
                fontSize = 9.sp,
                textAlign = TextAlign.Center,
                modifier = Modifier.padding(horizontal = 12.dp)
            )
        }
    } else {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                modifier = Modifier
                    .size(10.dp)
                    .clip(CircleShape)
                    .background(colorForKey(config.colorKey))
            )
            Spacer(modifier = Modifier.width(6.dp))
            Text(config.name, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = AppOnBackground)
        }
    }

    Spacer(modifier = Modifier.height(10.dp))
    Row(verticalAlignment = Alignment.CenterVertically) {
        StepperButton(label = "−", onClick = actions.onDecrement)
        Text(
            text = state.mulligans.toString(),
            fontSize = 30.sp,
            fontWeight = FontWeight.Bold,
            color = AppOnBackground,
            textAlign = TextAlign.Center,
            modifier = Modifier.widthIn(min = 64.dp)
        )
        StepperButton(label = "+", onClick = actions.onIncrement)
    }
    Text(
        stringResource(R.string.pregame_mulligans).uppercase(),
        fontSize = 9.sp,
        letterSpacing = 1.5.sp,
        color = AppMuted
    )
}

/** Seated member's name with a ✕ to free the seat; [trailing] goes after the name (the deck, on step 3). */
@Composable
private fun SeatHeader(name: String, onFreeSeat: () -> Unit, trailing: @Composable RowScope.() -> Unit = {}) {
    val freeSeatLabel = stringResource(R.string.pregame_free_seat)
    Row(
        verticalAlignment = Alignment.CenterVertically,
        modifier = Modifier.padding(horizontal = 8.dp)
    ) {
        Text(name, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = AppOnBackground, maxLines = 1)
        trailing()
        Spacer(modifier = Modifier.width(6.dp))
        Box(
            modifier = Modifier
                .size(24.dp)
                .clip(CircleShape)
                .background(Color.White.copy(alpha = 0.08f))
                .clickable(onClick = onFreeSeat)
                .semantics { contentDescription = freeSeatLabel },
            contentAlignment = Alignment.Center
        ) {
            Text("✕", fontSize = 11.sp, color = AppMuted)
        }
    }
}

@Composable
private fun StepperButton(label: String, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .size(40.dp)
            .clip(CircleShape)
            .background(Color.White.copy(alpha = 0.1f))
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Text(label, fontSize = 20.sp, color = Color.White)
    }
}

/**
 * The only control on this screen once seats are set: starts the game, picking the
 * starting player at random behind the scenes (shown via [GameTrackerScreen][com.vansid.tapeandocartones.presentation.screens.game.GameTrackerScreen]'s
 * starter banner right after) — matches the mockup's single centered play button, no
 * separate "shuffle" step.
 */
@Composable
private fun PlayButton(onClick: () -> Unit, modifier: Modifier = Modifier) {
    val label = stringResource(R.string.pregame_begin)
    Box(
        modifier = modifier
            .size(56.dp)
            .shadow(elevation = 12.dp, shape = CircleShape, clip = false)
            .clip(CircleShape)
            .background(AccentGradient)
            .clickable(onClick = onClick)
            .semantics { contentDescription = label },
        contentAlignment = Alignment.Center
    ) {
        Text("▶", color = AppBackgroundDeep, fontSize = 20.sp, modifier = Modifier.padding(start = 3.dp))
    }
}
