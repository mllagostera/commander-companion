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
import com.commandercompanion.data.remote.dto.PlaygroupDto
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

/** Casual: zero network, zero stats — the usual life tracker. Group: see [PlaygroupDto]. */
private enum class SetupMode { CASUAL, GROUP }

/**
 * In Group mode, seats aren't assigned here: this screen only picks the playgroup and
 * player count. Which member sits where, their deck, and mulligans are all chosen on
 * [com.commandercompanion.presentation.screens.pregame.PreGameScreen] instead, matching
 * the mockup ("Asiento, color y deck se eligen al empezar.") — Casual mode keeps
 * everything here since it has no accounts/decks to assign.
 */
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

    AppScreenBackground {
        // A single scrolling LazyColumn (header + player rows + the start button all as
        // items) rather than a fixed header above a weighted list: in landscape there's
        // much less height to work with, and a non-scrollable header could otherwise push
        // the player list and the start button off-screen with no way to reach them.
        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            item {
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

                if (mode == SetupMode.GROUP) {
                    Spacer(modifier = Modifier.height(16.dp))
                    PlaygroupPicker(
                        playgroups = viewModel.playgroups,
                        selected = selectedPlaygroup,
                        onSelected = { playgroup -> selectedPlaygroup = playgroup }
                    )
                }

                Spacer(modifier = Modifier.height(16.dp))
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
            }

            items(playerCount) { index ->
                PlayerConfigRow(
                    mode = mode,
                    name = names[index],
                    onNameChange = { names[index] = it },
                    selectedColorKey = colorKeys[index],
                    onColorSelected = { colorKeys[index] = it }
                )
            }

            item {
                Spacer(modifier = Modifier.height(4.dp))
                GradientButton(
                    text = stringResource(R.string.setup_start_game),
                    onClick = {
                        val configs = (0 until playerCount).map { index ->
                            PlayerConfig(
                                name = if (mode == SetupMode.GROUP) {
                                    defaultPlayerName.format(index + 1)
                                } else {
                                    names[index].ifBlank { defaultPlayerName.format(index + 1) }
                                },
                                colorKey = colorKeys[index]
                            )
                        }
                        val playgroupId = if (mode == SetupMode.GROUP) selectedPlaygroup?.id else null
                        onStartGame(UUID.randomUUID().toString(), encodePlayerConfigs(configs), playgroupId)
                    }
                )
            }
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
    onColorSelected: (String) -> Unit
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
            } else {
                Text(
                    stringResource(R.string.setup_group_seat_placeholder),
                    color = AppFaint,
                    fontSize = 12.sp
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
