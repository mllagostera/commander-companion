package com.commandercompanion.presentation.screens.setup

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import com.commandercompanion.R
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.PlaygroupDto
import com.commandercompanion.data.remote.dto.PlaygroupMemberDto
import com.commandercompanion.presentation.components.AppScreenBackground
import com.commandercompanion.presentation.components.GlassCard
import com.commandercompanion.presentation.components.GradientButton
import com.commandercompanion.presentation.components.PillSegmentedControl
import com.commandercompanion.presentation.components.SectionEyebrow
import com.commandercompanion.presentation.components.SelectableChip
import com.commandercompanion.presentation.components.SelectableCircle
import com.commandercompanion.presentation.navigation.PlayerConfig
import com.commandercompanion.presentation.navigation.encodePlayerConfigs
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppOutline
import com.commandercompanion.presentation.theme.PlayerColorPalette
import java.util.UUID

private const val MIN_PLAYERS = 2
private const val MAX_PLAYERS = 6

/** Casual: cero red, cero estadísticas — el life tracker de siempre. Grupo: ver [PlaygroupDto]. */
private enum class SetupMode { CASUAL, GROUP }

@Composable
fun PlayerSetupScreen(
    onStartGame: (gameId: String, playersEncoded: String, playgroupId: String?) -> Unit,
    viewModel: PlayerSetupViewModel = hiltViewModel()
) {
    var mode by remember { mutableStateOf(SetupMode.CASUAL) }
    var playerCount by remember { mutableIntStateOf(4) }
    val defaultPlayerName = stringResource(R.string.setup_default_player_name)
    val names = remember {
        mutableStateListOf(*Array(MAX_PLAYERS) { defaultPlayerName.format(it + 1) })
    }
    val colorKeys = remember {
        mutableStateListOf(*Array(MAX_PLAYERS) { PlayerColorPalette[it % PlayerColorPalette.size].first })
    }

    var selectedPlaygroup by remember { mutableStateOf<PlaygroupDto?>(null) }
    val assignedMembers = remember {
        mutableStateListOf(*arrayOfNulls<PlaygroupMemberDto>(MAX_PLAYERS))
    }
    val selectedDeckIds = remember {
        mutableStateListOf(*arrayOfNulls<String>(MAX_PLAYERS))
    }

    AppScreenBackground {
        Column(modifier = Modifier.fillMaxSize().padding(20.dp)) {
            Text(
                stringResource(R.string.setup_title),
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.SemiBold,
                fontSize = 20.sp
            )
            Spacer(modifier = Modifier.height(16.dp))

            PillSegmentedControl(
                options = listOf(
                    SetupMode.CASUAL to stringResource(R.string.setup_mode_casual),
                    SetupMode.GROUP to stringResource(R.string.setup_mode_group)
                ),
                selected = mode,
                onSelected = { mode = it }
            )
            Text(
                text = if (mode == SetupMode.CASUAL) {
                    stringResource(R.string.setup_mode_casual_description)
                } else {
                    stringResource(R.string.setup_mode_group_description)
                },
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                fontSize = 12.sp,
                modifier = Modifier.padding(top = 10.dp)
            )
            Spacer(modifier = Modifier.height(16.dp))

            if (mode == SetupMode.GROUP) {
                PlaygroupPicker(
                    playgroups = viewModel.playgroups,
                    selected = selectedPlaygroup,
                    onSelected = { playgroup ->
                        selectedPlaygroup = playgroup
                        assignedMembers.indices.forEach { assignedMembers[it] = null }
                    }
                )
                Spacer(modifier = Modifier.height(16.dp))
            }

            SectionEyebrow(stringResource(R.string.setup_players_label))
            Spacer(modifier = Modifier.height(8.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                (MIN_PLAYERS..MAX_PLAYERS).forEach { count ->
                    SelectableCircle(
                        label = count.toString(),
                        selected = playerCount == count,
                        onClick = { playerCount = count }
                    )
                }
            }

            Spacer(modifier = Modifier.height(12.dp))

            LazyColumn(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                items(playerCount) { index ->
                    val takenElsewhere = assignedMembers.filterIndexed { i, m -> i != index && m != null }
                        .mapNotNull { it?.userId }
                        .toSet()
                    PlayerConfigRow(
                        mode = mode,
                        name = names[index],
                        onNameChange = { names[index] = it },
                        selectedColorKey = colorKeys[index],
                        onColorSelected = { colorKeys[index] = it },
                        playgroup = selectedPlaygroup,
                        availableMembers = selectedPlaygroup?.members?.filter { it.userId !in takenElsewhere } ?: emptyList(),
                        ownUsername = viewModel.ownUsername,
                        assignedMember = assignedMembers[index],
                        onMemberSelected = { member ->
                            assignedMembers[index] = member
                            selectedDeckIds[index] = null
                            val playgroupId = selectedPlaygroup?.id
                            if (member != null && playgroupId != null) {
                                viewModel.loadMemberDecks(playgroupId, member.userId)
                            }
                        },
                        memberDecks = assignedMembers[index]?.let { viewModel.decksFor(it.userId) } ?: emptyList(),
                        selectedDeckId = selectedDeckIds[index],
                        onDeckSelected = { selectedDeckIds[index] = it }
                    )
                }
            }

            Spacer(modifier = Modifier.height(8.dp))
            GradientButton(
                text = stringResource(R.string.setup_start_game),
                onClick = {
                    val configs = (0 until playerCount).map { index ->
                        val member = if (mode == SetupMode.GROUP) assignedMembers[index] else null
                        PlayerConfig(
                            name = member?.username ?: names[index].ifBlank { defaultPlayerName.format(index + 1) },
                            colorKey = colorKeys[index],
                            assignedUserId = member?.userId,
                            assignedUsername = member?.username,
                            deckId = if (member != null) selectedDeckIds[index] else null
                        )
                    }
                    val playgroupId = if (mode == SetupMode.GROUP) selectedPlaygroup?.id else null
                    onStartGame(UUID.randomUUID().toString(), encodePlayerConfigs(configs), playgroupId)
                }
            )
        }
    }
}

@Composable
private fun PlaygroupPicker(
    playgroups: List<PlaygroupDto>,
    selected: PlaygroupDto?,
    onSelected: (PlaygroupDto) -> Unit
) {
    Column {
        SectionEyebrow(stringResource(R.string.setup_group_label))
        Spacer(modifier = Modifier.height(8.dp))
        if (playgroups.isEmpty()) {
            Text(
                stringResource(R.string.setup_no_playgroups),
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                fontSize = 12.sp
            )
        } else {
            LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                items(playgroups) { playgroup ->
                    SelectableChip(
                        label = playgroup.name,
                        selected = playgroup.id == selected?.id,
                        onClick = { onSelected(playgroup) }
                    )
                }
            }
        }
    }
}

@Composable
private fun PlayerConfigRow(
    mode: SetupMode,
    name: String,
    onNameChange: (String) -> Unit,
    selectedColorKey: String,
    onColorSelected: (String) -> Unit,
    playgroup: PlaygroupDto?,
    availableMembers: List<PlaygroupMemberDto>,
    ownUsername: String?,
    assignedMember: PlaygroupMemberDto?,
    onMemberSelected: (PlaygroupMemberDto?) -> Unit,
    memberDecks: List<DeckDto>,
    selectedDeckId: String?,
    onDeckSelected: (String) -> Unit
) {
    GlassCard(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(20.dp)) {
        Column(modifier = Modifier.fillMaxWidth()) {
            if (mode == SetupMode.CASUAL) {
                OutlinedTextField(
                    value = name,
                    onValueChange = onNameChange,
                    label = { Text(stringResource(R.string.setup_name_label)) },
                    singleLine = true,
                    shape = RoundedCornerShape(percent = 50),
                    colors = OutlinedTextFieldDefaults.colors(
                        unfocusedBorderColor = AppOutline,
                        focusedBorderColor = MaterialTheme.colorScheme.primary
                    ),
                    modifier = Modifier.fillMaxWidth()
                )
            } else {
                MemberPicker(
                    playgroup = playgroup,
                    availableMembers = availableMembers,
                    ownUsername = ownUsername,
                    assignedMember = assignedMember,
                    onMemberSelected = onMemberSelected
                )
            }

            Spacer(modifier = Modifier.height(10.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                PlayerColorPalette.forEach { (key, color) ->
                    ColorSwatch(
                        color = color,
                        selected = key == selectedColorKey,
                        onClick = { onColorSelected(key) }
                    )
                }
            }

            if (mode == SetupMode.GROUP && assignedMember != null) {
                Spacer(modifier = Modifier.height(10.dp))
                if (memberDecks.isEmpty()) {
                    Text(
                        stringResource(R.string.setup_member_no_decks, assignedMember.username),
                        color = AppFaint,
                        fontSize = 12.sp
                    )
                } else {
                    Text(stringResource(R.string.setup_which_deck), color = MaterialTheme.colorScheme.onSurfaceVariant, fontSize = 12.sp)
                    Spacer(modifier = Modifier.height(6.dp))
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        items(memberDecks) { deck ->
                            SelectableChip(
                                label = deck.name,
                                selected = deck.id == selectedDeckId,
                                onClick = { onDeckSelected(deck.id) }
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun MemberPicker(
    playgroup: PlaygroupDto?,
    availableMembers: List<PlaygroupMemberDto>,
    ownUsername: String?,
    assignedMember: PlaygroupMemberDto?,
    onMemberSelected: (PlaygroupMemberDto?) -> Unit
) {
    Column {
        Text(stringResource(R.string.setup_seat_label), color = MaterialTheme.colorScheme.onSurfaceVariant, fontSize = 12.sp)
        Spacer(modifier = Modifier.height(6.dp))
        if (playgroup == null) {
            Text(
                stringResource(R.string.setup_pick_group_first),
                color = AppFaint,
                fontSize = 12.sp
            )
            return@Column
        }
        val youSuffix = stringResource(R.string.common_you_suffix)
        LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            item {
                SelectableChip(
                    label = stringResource(R.string.setup_guest),
                    selected = assignedMember == null,
                    onClick = { onMemberSelected(null) }
                )
            }
            items(availableMembers) { member ->
                val label = if (member.username == ownUsername) "${member.username} $youSuffix" else member.username
                SelectableChip(
                    label = label,
                    selected = member.userId == assignedMember?.userId,
                    onClick = { onMemberSelected(member) }
                )
            }
        }
    }
}

@Composable
private fun ColorSwatch(color: Color, selected: Boolean, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .size(26.dp)
            .clip(CircleShape)
            .background(color)
            .border(
                width = if (selected) 3.dp else 0.dp,
                color = MaterialTheme.colorScheme.primary,
                shape = CircleShape
            )
            .clickable(onClick = onClick)
    )
}
