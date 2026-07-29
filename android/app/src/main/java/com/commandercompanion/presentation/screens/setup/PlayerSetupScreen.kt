package com.commandercompanion.presentation.screens.setup

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.PlaygroupDto
import com.commandercompanion.data.remote.dto.PlaygroupMemberDto
import com.commandercompanion.presentation.navigation.PlayerConfig
import com.commandercompanion.presentation.navigation.encodePlayerConfigs
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
    val names = remember {
        mutableStateListOf(*Array(MAX_PLAYERS) { "Jugador ${it + 1}" })
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

    Column(modifier = Modifier.fillMaxSize().padding(16.dp)) {
        Text("Nueva partida", style = MaterialTheme.typography.headlineMedium)
        Spacer(modifier = Modifier.height(16.dp))

        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            FilterChip(
                selected = mode == SetupMode.CASUAL,
                onClick = { mode = SetupMode.CASUAL },
                label = { Text("Casual") }
            )
            FilterChip(
                selected = mode == SetupMode.GROUP,
                onClick = { mode = SetupMode.GROUP },
                label = { Text("Grupo") }
            )
        }
        Text(
            text = if (mode == SetupMode.CASUAL) {
                "Sin cuentas ni estadísticas: solo trackear la partida en este dispositivo."
            } else {
                "Asigná asientos a miembros de tu grupo: sus estadísticas quedan reales al terminar."
            },
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(top = 4.dp)
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

        Text("Jugadores", style = MaterialTheme.typography.titleSmall)
        Row(
            modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            (MIN_PLAYERS..MAX_PLAYERS).forEach { count ->
                FilterChip(
                    selected = playerCount == count,
                    onClick = { playerCount = count },
                    label = { Text(count.toString()) }
                )
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        LazyColumn(modifier = Modifier.weight(1f)) {
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

        Button(
            onClick = {
                val configs = (0 until playerCount).map { index ->
                    val member = if (mode == SetupMode.GROUP) assignedMembers[index] else null
                    PlayerConfig(
                        name = member?.username ?: names[index].ifBlank { "Jugador ${index + 1}" },
                        colorKey = colorKeys[index],
                        assignedUserId = member?.userId,
                        assignedUsername = member?.username,
                        deckId = if (member != null) selectedDeckIds[index] else null
                    )
                }
                val playgroupId = if (mode == SetupMode.GROUP) selectedPlaygroup?.id else null
                onStartGame(UUID.randomUUID().toString(), encodePlayerConfigs(configs), playgroupId)
            },
            modifier = Modifier.fillMaxWidth().height(56.dp)
        ) {
            Text("EMPEZAR PARTIDA", style = MaterialTheme.typography.titleMedium)
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
        Text("Grupo", style = MaterialTheme.typography.titleSmall)
        Spacer(modifier = Modifier.height(8.dp))
        if (playgroups.isEmpty()) {
            Text(
                "No sos miembro de ningún grupo todavía. Creá uno desde el cliente web.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        } else {
            LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                items(playgroups) { playgroup ->
                    FilterChip(
                        selected = playgroup.id == selected?.id,
                        onClick = { onSelected(playgroup) },
                        label = { Text(playgroup.name) }
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
    Column(modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp)) {
        if (mode == SetupMode.CASUAL) {
            OutlinedTextField(
                value = name,
                onValueChange = onNameChange,
                label = { Text("Nombre") },
                singleLine = true,
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

        Spacer(modifier = Modifier.height(4.dp))
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
            Spacer(modifier = Modifier.height(8.dp))
            if (memberDecks.isEmpty()) {
                Text(
                    "${assignedMember.username} todavía no tiene decks: este asiento no va a quedar " +
                        "guardado en sus estadísticas.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            } else {
                Text("¿Con qué deck juega?", style = MaterialTheme.typography.labelMedium)
                Spacer(modifier = Modifier.height(4.dp))
                LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    items(memberDecks) { deck ->
                        FilterChip(
                            selected = deck.id == selectedDeckId,
                            onClick = { onDeckSelected(deck.id) },
                            label = { Text(deck.name) }
                        )
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
        Text("Asiento", style = MaterialTheme.typography.labelMedium)
        Spacer(modifier = Modifier.height(4.dp))
        if (playgroup == null) {
            Text(
                "Elegí un grupo arriba para poder asignar jugadores.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            return@Column
        }
        LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            item {
                FilterChip(
                    selected = assignedMember == null,
                    onClick = { onMemberSelected(null) },
                    label = { Text("Invitado") }
                )
            }
            items(availableMembers) { member ->
                val label = if (member.username == ownUsername) "${member.username} (vos)" else member.username
                FilterChip(
                    selected = member.userId == assignedMember?.userId,
                    onClick = { onMemberSelected(member) },
                    label = { Text(label) }
                )
            }
        }
    }
}

@Composable
private fun ColorSwatch(color: Color, selected: Boolean, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .size(32.dp)
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
