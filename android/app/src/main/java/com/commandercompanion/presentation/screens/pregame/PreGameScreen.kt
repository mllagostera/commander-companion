package com.commandercompanion.presentation.screens.pregame

import android.content.res.Configuration
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
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
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.commandercompanion.R
import com.commandercompanion.presentation.components.GradientButton
import com.commandercompanion.presentation.components.GradientOutlineButton
import com.commandercompanion.presentation.components.RotateDevicePrompt
import com.commandercompanion.presentation.navigation.PlayerConfig
import com.commandercompanion.presentation.navigation.decodePlayerConfigs
import com.commandercompanion.presentation.navigation.encodePlayerConfigs
import com.commandercompanion.presentation.theme.AccentSoft
import com.commandercompanion.presentation.theme.AppBackgroundDeep
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppOnBackground
import com.commandercompanion.presentation.theme.colorForKey
import kotlin.random.Random

/**
 * Only makes sense played in landscape (each seat faces its own edge of the
 * "table"), so in portrait we show the prompt to rotate the device — real orientation
 * via `LocalConfiguration`, not a simulated timer.
 */
@Composable
fun PreGameScreen(
    playersEncoded: String,
    onContinue: (playersEncoded: String, startingPlayerSeat: Int) -> Unit
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
    var startingSeat by rememberSaveable { mutableIntStateOf(-1) }

    val isLandscape = LocalConfiguration.current.orientation == Configuration.ORIENTATION_LANDSCAPE

    Box(modifier = Modifier.fillMaxSize().background(AppBackgroundDeep)) {
        if (!isLandscape) {
            RotateDevicePrompt(message = stringResource(R.string.pregame_rotate_prompt))
        } else {
            SeatGrid(
                configs = configs,
                mulligans = mulligans,
                startingSeat = startingSeat,
                onIncrement = { index -> mulligans[index] = mulligans[index] + 1 },
                onDecrement = { index -> mulligans[index] = (mulligans[index] - 1).coerceAtLeast(0) }
            )

            StarterOverlay(
                startingSeat = startingSeat,
                configs = configs,
                onSortear = { startingSeat = Random.nextInt(configs.size) },
                onEmpezar = {
                    val updatedConfigs = configs.mapIndexed { index, config -> config.copy(mulligans = mulligans[index]) }
                    onContinue(encodePlayerConfigs(updatedConfigs), startingSeat)
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
    startingSeat: Int,
    onIncrement: (Int) -> Unit,
    onDecrement: (Int) -> Unit
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
                    isStarter = startingSeat == index,
                    onIncrement = { onIncrement(index) },
                    onDecrement = { onDecrement(index) },
                    rotated = true,
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
                    isStarter = startingSeat == index,
                    onIncrement = { onIncrement(index) },
                    onDecrement = { onDecrement(index) },
                    rotated = false,
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
    isStarter: Boolean,
    onIncrement: () -> Unit,
    onDecrement: () -> Unit,
    rotated: Boolean,
    modifier: Modifier = Modifier
) {
    Box(
        modifier = modifier
            .then(if (rotated) Modifier.rotate(180f) else Modifier)
            .clip(RoundedCornerShape(22.dp))
            .background(
                if (isStarter) {
                    Brush.linearGradient(listOf(AccentSoft.copy(alpha = 0.35f), Color.White.copy(alpha = 0.05f)))
                } else {
                    Brush.linearGradient(listOf(Color.White.copy(alpha = 0.04f), Color.White.copy(alpha = 0.04f)))
                }
            )
    ) {
        Column(
            modifier = Modifier.fillMaxSize().padding(10.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            Text(stringResource(R.string.pregame_seat_label, seatIndex + 1), fontSize = 10.sp, color = AppFaint)
            Spacer(modifier = Modifier.height(6.dp))
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
            Spacer(modifier = Modifier.height(10.dp))
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

@Composable
private fun StarterOverlay(
    startingSeat: Int,
    configs: List<PlayerConfig>,
    onSortear: () -> Unit,
    onEmpezar: () -> Unit,
    modifier: Modifier = Modifier
) {
    Column(
        modifier = modifier
            .width(160.dp)
            .clip(RoundedCornerShape(20.dp))
            .background(AppBackgroundDeep.copy(alpha = 0.92f))
            .border(1.dp, AccentSoft.copy(alpha = 0.35f), RoundedCornerShape(20.dp))
            .padding(horizontal = 14.dp, vertical = 12.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        Text(
            text = if (startingSeat >= 0) {
                stringResource(R.string.pregame_starts, configs[startingSeat].name)
            } else {
                stringResource(R.string.pregame_who_starts)
            },
            color = AccentSoft,
            fontSize = 10.sp,
            textAlign = TextAlign.Center
        )
        GradientOutlineButton(text = stringResource(R.string.pregame_shuffle), onClick = onSortear)
        GradientButton(text = stringResource(R.string.pregame_begin), onClick = onEmpezar)
    }
}
