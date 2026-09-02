package com.commandercompanion.presentation.screens.pregame

import android.content.res.Configuration
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.listSaver
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshots.SnapshotStateList
import androidx.compose.runtime.toMutableStateList
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import com.commandercompanion.R
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.PlaygroupMemberDto
import com.commandercompanion.presentation.components.DeckArtChip
import com.commandercompanion.presentation.components.RotateDevicePrompt
import com.commandercompanion.presentation.components.SelectableChip
import com.commandercompanion.presentation.navigation.PlayerConfig
import com.commandercompanion.presentation.navigation.decodePlayerConfigs
import com.commandercompanion.presentation.navigation.encodePlayerConfigs
import com.commandercompanion.presentation.theme.AccentGradient
import com.commandercompanion.presentation.theme.AppBackgroundDeep
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppOnBackground
import com.commandercompanion.presentation.theme.colorForKey
import kotlin.random.Random

private fun stringListSaver() = listSaver<SnapshotStateList<String?>, String?>(
    save = { it.toList() },
    restore = { it.toMutableStateList() }
)

/**
 * Only makes sense played in landscape (each seat faces its own edge of the
 * "table"), so in portrait we show the prompt to rotate the device — real orientation
 * via `LocalConfiguration`, not a simulated timer.
 *
 * In Group mode ([playgroupId] non-null), this is also where seats get assigned to
 * playgroup members and decks — [PlayerSetupScreen][com.commandercompanion.presentation.screens.setup.PlayerSetupScreen]
 * only picks the playgroup and player count, matching the mockup ("Asiento, color y
 * deck se eligen al empezar."). Casual mode ([playgroupId] null) keeps the seat/color
 * already chosen in Setup and only adds mulligans + the starting-player draw here, same
 * as before.
 */
@Composable
fun PreGameScreen(
    playersEncoded: String,
    playgroupId: String?,
    onContinue: (playersEncoded: String, startingPlayerSeat: Int) -> Unit,
    viewModel: PreGameViewModel = hiltViewModel()
) {
    val configs = remember { decodePlayerConfigs(playersEncoded) }
    val mulliganListSaver = remember {
        listSaver<SnapshotStateList<Int>, Int>(
            save = { it.toList() },
            restore = { it.toMutableStateList() }
        )
    }
    val mulligans = rememberSaveable(saver = mulliganListSaver) {
        mutableStateListOf(*IntArray(configs.size) { 0 }.toTypedArray())
    }

    // Group mode only: which member (or "" for a Guest) sits at each seat, and their deck.
    // A null entry means the seat hasn't been assigned yet — the member picker keeps showing.
    val seatMemberUserIds = rememberSaveable(saver = stringListSaver()) {
        mutableStateListOf(*arrayOfNulls<String>(configs.size))
    }
    val seatMemberUsernames = rememberSaveable(saver = stringListSaver()) {
        mutableStateListOf(*arrayOfNulls<String>(configs.size))
    }
    val seatDeckIds = rememberSaveable(saver = stringListSaver()) {
        mutableStateListOf(*arrayOfNulls<String>(configs.size))
    }

    LaunchedEffect(playgroupId) {
        if (playgroupId != null) viewModel.loadPlaygroup(playgroupId)
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
    val isLandscape = LocalConfiguration.current.orientation == Configuration.ORIENTATION_LANDSCAPE

    Box(modifier = Modifier.fillMaxSize().background(AppBackgroundDeep)) {
        if (!isLandscape) {
            RotateDevicePrompt(message = stringResource(R.string.pregame_rotate_prompt))
        } else {
            SeatGrid(
                configs = configs,
                mulligans = mulligans,
                onIncrement = { index -> mulligans[index] = mulligans[index] + 1 },
                onDecrement = { index -> mulligans[index] = (mulligans[index] - 1).coerceAtLeast(0) },
                playgroupId = playgroupId,
                availableMembers = viewModel.playgroup?.members.orEmpty(),
                ownUsername = viewModel.ownUsername,
                seatMemberUserIds = seatMemberUserIds,
                seatMemberUsernames = seatMemberUsernames,
                onMemberSelected = { index, member ->
                    seatMemberUserIds[index] = member?.userId ?: ""
                    seatMemberUsernames[index] = member?.username
                    if (member != null && playgroupId != null) {
                        viewModel.loadMemberDecks(playgroupId, member.userId)
                    }
                },
                memberDecksFor = { userId -> viewModel.decksFor(userId) },
                seatDeckIds = seatDeckIds,
                onDeckSelected = { index, deckId -> seatDeckIds[index] = deckId }
            )

            PlayButton(
                onClick = {
                    val updatedConfigs = configs.mapIndexed { index, config ->
                        if (playgroupId != null) {
                            val userId = seatMemberUserIds[index]?.takeIf { it.isNotEmpty() }
                            config.copy(
                                name = displayName(index),
                                mulligans = mulligans[index],
                                assignedUserId = userId,
                                assignedUsername = seatMemberUsernames[index],
                                deckId = if (userId != null) seatDeckIds[index] else null
                            )
                        } else {
                            config.copy(mulligans = mulligans[index])
                        }
                    }
                    onContinue(encodePlayerConfigs(updatedConfigs), Random.nextInt(configs.size))
                },
                modifier = Modifier.align(Alignment.Center)
            )
        }
    }
}

@Composable
private fun SeatGrid(
    configs: List<PlayerConfig>,
    mulligans: List<Int>,
    onIncrement: (Int) -> Unit,
    onDecrement: (Int) -> Unit,
    playgroupId: String?,
    availableMembers: List<PlaygroupMemberDto>,
    ownUsername: String?,
    seatMemberUserIds: List<String?>,
    seatMemberUsernames: List<String?>,
    onMemberSelected: (index: Int, member: PlaygroupMemberDto?) -> Unit,
    memberDecksFor: (userId: String) -> List<DeckDto>,
    seatDeckIds: List<String?>,
    onDeckSelected: (index: Int, deckId: String?) -> Unit
) {
    val topCount = (configs.size + 1) / 2
    Column(
        modifier = Modifier.fillMaxSize().padding(4.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp)
    ) {
        Row(
            modifier = Modifier.weight(1f).fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(4.dp)
        ) {
            (0 until topCount).forEach { index ->
                SeatCard(
                    seatIndex = index,
                    config = configs[index],
                    mulligans = mulligans[index],
                    onIncrement = { onIncrement(index) },
                    onDecrement = { onDecrement(index) },
                    rotated = true,
                    playgroupId = playgroupId,
                    availableMembers = availableMembers.filter { it.userId !in seatMemberUserIds.filterIndexed { i, _ -> i != index } },
                    ownUsername = ownUsername,
                    assignedUserId = seatMemberUserIds[index],
                    assignedUsername = seatMemberUsernames[index],
                    onMemberSelected = { member -> onMemberSelected(index, member) },
                    memberDecks = seatMemberUserIds[index]?.takeIf { it.isNotEmpty() }?.let(memberDecksFor).orEmpty(),
                    selectedDeckId = seatDeckIds[index],
                    onDeckSelected = { deckId -> onDeckSelected(index, deckId) },
                    modifier = Modifier.weight(1f).fillMaxHeight()
                )
            }
        }
        Row(
            modifier = Modifier.weight(1f).fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(4.dp)
        ) {
            (topCount until configs.size).forEach { index ->
                SeatCard(
                    seatIndex = index,
                    config = configs[index],
                    mulligans = mulligans[index],
                    onIncrement = { onIncrement(index) },
                    onDecrement = { onDecrement(index) },
                    rotated = false,
                    playgroupId = playgroupId,
                    availableMembers = availableMembers.filter { it.userId !in seatMemberUserIds.filterIndexed { i, _ -> i != index } },
                    ownUsername = ownUsername,
                    assignedUserId = seatMemberUserIds[index],
                    assignedUsername = seatMemberUsernames[index],
                    onMemberSelected = { member -> onMemberSelected(index, member) },
                    memberDecks = seatMemberUserIds[index]?.takeIf { it.isNotEmpty() }?.let(memberDecksFor).orEmpty(),
                    selectedDeckId = seatDeckIds[index],
                    onDeckSelected = { deckId -> onDeckSelected(index, deckId) },
                    modifier = Modifier.weight(1f).fillMaxHeight()
                )
            }
        }
    }
}

@Composable
private fun SeatCard(
    seatIndex: Int,
    config: PlayerConfig,
    mulligans: Int,
    onIncrement: () -> Unit,
    onDecrement: () -> Unit,
    rotated: Boolean,
    playgroupId: String?,
    availableMembers: List<PlaygroupMemberDto>,
    ownUsername: String?,
    assignedUserId: String?,
    assignedUsername: String?,
    onMemberSelected: (PlaygroupMemberDto?) -> Unit,
    memberDecks: List<DeckDto>,
    selectedDeckId: String?,
    onDeckSelected: (String?) -> Unit,
    modifier: Modifier = Modifier
) {
    Box(
        modifier = modifier
            .then(if (rotated) Modifier.rotate(180f) else Modifier)
            .clip(RoundedCornerShape(22.dp))
            .background(Brush.linearGradient(listOf(Color.White.copy(alpha = 0.04f), Color.White.copy(alpha = 0.04f))))
    ) {
        Column(
            modifier = Modifier.fillMaxSize().padding(8.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            Text(stringResource(R.string.pregame_seat_label, seatIndex + 1), fontSize = 10.sp, color = AppFaint)
            Spacer(modifier = Modifier.height(6.dp))

            if (playgroupId == null) {
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
            } else if (assignedUserId == null) {
                GroupSeatMemberPicker(
                    availableMembers = availableMembers,
                    ownUsername = ownUsername,
                    onMemberSelected = onMemberSelected
                )
            } else {
                GroupSeatAssigned(
                    assignedUsername = assignedUsername,
                    memberDecks = memberDecks,
                    selectedDeckId = selectedDeckId,
                    onDeckSelected = onDeckSelected
                )
            }

            Spacer(modifier = Modifier.height(8.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(stringResource(R.string.pregame_mulligans), fontSize = 9.sp, color = AppFaint)
                Spacer(modifier = Modifier.width(8.dp))
                StepperButton(label = "−", onClick = onDecrement)
                Text(
                    text = mulligans.toString(),
                    fontSize = 11.sp,
                    color = AppOnBackground,
                    modifier = Modifier.padding(horizontal = 8.dp)
                )
                StepperButton(label = "+", onClick = onIncrement)
            }
        }
    }
}

/** Group mode, seat not assigned yet: pick a Guest or an available playgroup member — locks in once tapped. */
@Composable
private fun GroupSeatMemberPicker(
    availableMembers: List<PlaygroupMemberDto>,
    ownUsername: String?,
    onMemberSelected: (PlaygroupMemberDto?) -> Unit
) {
    val youSuffix = stringResource(R.string.common_you_suffix)
    FlowRow(
        horizontalArrangement = Arrangement.spacedBy(4.dp),
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

/** Group mode, seat locked to a member (or Guest): show who's seated and their deck. */
@Composable
private fun GroupSeatAssigned(
    assignedUsername: String?,
    memberDecks: List<DeckDto>,
    selectedDeckId: String?,
    onDeckSelected: (String?) -> Unit
) {
    Text(
        assignedUsername ?: stringResource(R.string.setup_guest),
        fontSize = 13.sp,
        fontWeight = FontWeight.SemiBold,
        color = AppOnBackground
    )

    // Guests have no account to attach a deck to — only real members pick one.
    if (assignedUsername == null) return

    Spacer(modifier = Modifier.height(6.dp))
    val selectedDeck = memberDecks.firstOrNull { it.id == selectedDeckId }
    when {
        selectedDeck != null -> DeckArtChip(
            name = selectedDeck.name,
            imageUrl = selectedDeck.imageUrl,
            selected = true,
            onClick = { onDeckSelected(null) },
            width = 92.dp,
            height = 60.dp
        )
        memberDecks.isEmpty() -> Text(
            stringResource(R.string.setup_member_no_decks, assignedUsername),
            color = AppFaint,
            fontSize = 9.sp,
            textAlign = TextAlign.Center
        )
        else -> LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            items(memberDecks) { deck ->
                DeckArtChip(
                    name = deck.name,
                    imageUrl = deck.imageUrl,
                    selected = false,
                    onClick = { onDeckSelected(deck.id) },
                    width = 60.dp,
                    height = 40.dp
                )
            }
        }
    }
}

@Composable
private fun StepperButton(label: String, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .size(18.dp)
            .clip(CircleShape)
            .background(Color.White.copy(alpha = 0.1f))
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Text(label, fontSize = 10.sp, color = Color.White)
    }
}

/**
 * The only control on this screen once seats are set: starts the game, picking the
 * starting player at random behind the scenes (shown via [GameTrackerScreen][com.commandercompanion.presentation.screens.game.GameTrackerScreen]'s
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
