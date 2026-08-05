package com.commandercompanion.presentation.components

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil3.compose.AsyncImage
import com.commandercompanion.presentation.theme.AccentGradient
import com.commandercompanion.presentation.theme.AccentSoft
import com.commandercompanion.presentation.theme.AppBackground
import com.commandercompanion.presentation.theme.AppBackgroundDeep
import com.commandercompanion.presentation.theme.AppFaint
import com.commandercompanion.presentation.theme.AppOnBackground
import com.commandercompanion.presentation.theme.AppOutline
import com.commandercompanion.presentation.theme.TitleGradient

private val PillShape = RoundedCornerShape(percent = 50)

/** Rotated MTG-card mark used as the app's brand identity on Login/Dashboard. */
@Composable
fun AppLogoMark(
    modifier: Modifier = Modifier,
    width: Dp = 46.dp,
    height: Dp = 64.dp
) {
    Box(
        modifier = modifier
            .size(width, height)
            .rotate(-6f)
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .clip(RoundedCornerShape(8.dp))
                .background(Brush.linearGradient(listOf(Color(0xFF4C1D95), Color(0xFF2F2159))))
        )
        Box(
            modifier = Modifier
                .padding(4.dp)
                .fillMaxSize()
                .clip(RoundedCornerShape(6.dp))
                .background(Color(0xFFE9E4FB))
        ) {
            Box(
                modifier = Modifier
                    .padding(4.dp)
                    .fillMaxSize()
                    .clip(RoundedCornerShape(4.dp))
                    .background(AccentGradient)
            )
        }
    }
}

/** Gradient title text ("Commander Companion"), matching the mockup's brand headline. */
@Composable
fun GradientTitle(
    text: String,
    modifier: Modifier = Modifier,
    fontSize: TextUnit = 20.sp
) {
    Text(
        text = text,
        modifier = modifier,
        style = TextStyle(
            brush = TitleGradient,
            fontWeight = FontWeight.SemiBold,
            fontSize = fontSize
        )
    )
}

/** Primary CTA: full-width gradient pill button ("NUEVA PARTIDA", "Iniciar sesión", ...). */
@Composable
fun GradientButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    content: (@Composable () -> Unit)? = null
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .shadow(elevation = if (enabled) 12.dp else 0.dp, shape = PillShape, clip = false)
            .clip(PillShape)
            .background(if (enabled) AccentGradient else Brush.horizontalGradient(listOf(AppFaint, AppFaint)))
            .clickable(enabled = enabled, onClick = onClick)
            .padding(vertical = 16.dp),
        contentAlignment = Alignment.Center
    ) {
        if (content != null) {
            content()
        } else {
            Text(
                text = text,
                color = AppBackgroundDeep,
                fontWeight = FontWeight.Bold,
                fontSize = 14.sp,
                letterSpacing = 0.5.sp
            )
        }
    }
}

/** Secondary CTA: translucent outlined pill ("HISTORIAL", "Continuar con Google"). */
@Composable
fun GradientOutlineButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .clip(PillShape)
            .background(Color.White.copy(alpha = 0.03f))
            .border(1.dp, AppOutline, PillShape)
            .clickable(enabled = enabled, onClick = onClick)
            .padding(vertical = 14.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(text = text, color = AppOnBackground, fontWeight = FontWeight.SemiBold, fontSize = 13.sp)
    }
}

/** A rounded translucent panel — the "glass card" surface used throughout the mockup. */
@Composable
fun GlassCard(
    modifier: Modifier = Modifier,
    shape: Shape = RoundedCornerShape(20.dp),
    contentPadding: PaddingValues = PaddingValues(16.dp),
    content: @Composable () -> Unit
) {
    Box(
        modifier = modifier
            .clip(shape)
            .background(Color.White.copy(alpha = 0.03f))
            .border(1.dp, AppOutline, shape)
            .padding(contentPadding)
    ) {
        content()
    }
}

/** Two-option pill switch ("Casual" / "Grupo"). */
@Composable
fun <T> PillSegmentedControl(
    options: List<Pair<T, String>>,
    selected: T,
    onSelected: (T) -> Unit,
    modifier: Modifier = Modifier
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .clip(PillShape)
            .background(Color.White.copy(alpha = 0.04f))
            .border(1.dp, AppOutline, PillShape)
            .padding(4.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        options.forEach { (value, label) ->
            val isSelected = value == selected
            Box(
                modifier = Modifier
                    .weight(1f)
                    .clip(PillShape)
                    .background(if (isSelected) AccentGradient else Brush.horizontalGradient(listOf(Color.Transparent, Color.Transparent)))
                    .clickable { onSelected(value) }
                    .padding(vertical = 9.dp),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = label,
                    color = if (isSelected) AppBackgroundDeep else MaterialTheme.colorScheme.onSurfaceVariant,
                    fontWeight = FontWeight.SemiBold,
                    fontSize = 12.sp
                )
            }
        }
    }
}

/** Small pill chip used for selectable lists (playgroups, seats, decks). */
@Composable
fun SelectableChip(
    label: String,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    Box(
        modifier = modifier
            .clip(PillShape)
            .background(if (selected) AccentGradient else SolidColor(Color.White.copy(alpha = 0.05f)))
            .then(if (!selected) Modifier.border(1.dp, AppOutline, PillShape) else Modifier)
            .clickable(onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 8.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = label,
            color = if (selected) AppBackgroundDeep else AppOnBackground,
            fontWeight = FontWeight.SemiBold,
            fontSize = 12.sp
        )
    }
}

/**
 * Selectable deck card for the pregame deck picker: shows the commander's art crop
 * ([com.commandercompanion.data.remote.dto.DeckDto.imageUrl], populated for Moxfield-imported
 * decks) with the deck name captioned over a bottom scrim, matching the mockup's deck thumbnails.
 * Manually-created decks have no art yet, so this falls back to a plain [SelectableChip]-style
 * card with just the name.
 *
 * [width]/[height] default to a comfortable size; pass smaller values (e.g. in a cramped
 * pregame seat tile) to fit tighter layouts — same visual, just scaled down.
 */
@Composable
fun DeckArtChip(
    name: String,
    imageUrl: String?,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    width: Dp = 108.dp,
    height: Dp = 72.dp
) {
    val shape = RoundedCornerShape(14.dp)
    val background = when {
        imageUrl != null -> SolidColor(Color.Black)
        selected -> AccentGradient
        else -> SolidColor(Color.White.copy(alpha = 0.05f))
    }
    Box(
        modifier = modifier
            .size(width = width, height = height)
            .clip(shape)
            .background(background)
            .border(if (selected) 2.dp else 1.dp, if (selected) AccentSoft else AppOutline, shape)
            .clickable(onClick = onClick)
    ) {
        if (imageUrl != null) {
            AsyncImage(
                model = imageUrl,
                contentDescription = name,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize()
            )
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(Brush.verticalGradient(listOf(Color.Transparent, Color.Black.copy(alpha = 0.85f))))
            )
            Text(
                text = name,
                color = Color.White,
                fontWeight = FontWeight.SemiBold,
                fontSize = 10.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier
                    .align(Alignment.BottomStart)
                    .padding(horizontal = 8.dp, vertical = 6.dp)
            )
        } else {
            Text(
                text = name,
                color = if (selected) AppBackgroundDeep else AppOnBackground,
                fontWeight = FontWeight.SemiBold,
                fontSize = 11.sp,
                textAlign = TextAlign.Center,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.align(Alignment.Center).padding(8.dp)
            )
        }
    }
}

/**
 * Plain (non-interactive) square deck thumbnail — commander art crop when [imageUrl] is set,
 * else a gradient tile with the commander's first letter. Mirrors web's `DeckArt.vue`. Unlike
 * [DeckArtChip], carries no selection border/click/name-overlay, so it fits compact rows like
 * a statistics card.
 */
@Composable
fun DeckThumbnail(
    commander: String,
    imageUrl: String?,
    modifier: Modifier = Modifier,
    size: Dp = 64.dp
) {
    val shape = RoundedCornerShape(14.dp)
    Box(
        modifier = modifier
            .size(size)
            .clip(shape)
            .border(1.dp, AppOutline, shape)
    ) {
        if (imageUrl != null) {
            AsyncImage(
                model = imageUrl,
                contentDescription = commander,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize()
            )
        } else {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(
                        Brush.linearGradient(
                            listOf(AccentSoft.copy(alpha = 0.35f), AccentSoft.copy(alpha = 0.15f))
                        )
                    ),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = commander.firstOrNull()?.uppercase() ?: "?",
                    color = AppFaint,
                    fontWeight = FontWeight.Bold,
                    fontSize = 20.sp
                )
            }
        }
    }
}

/** Circular number/index picker, e.g. player-count selector. */
@Composable
fun SelectableCircle(
    label: String,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    size: Dp = 36.dp
) {
    Box(
        modifier = modifier
            .size(size)
            .clip(CircleShape)
            .background(if (selected) AccentGradient else SolidColor(Color.White.copy(alpha = 0.05f)))
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = label,
            color = if (selected) AppBackgroundDeep else AppOnBackground,
            fontWeight = FontWeight.SemiBold,
            fontSize = 13.sp
        )
    }
}

/** Small circular outlined button, e.g. the "‹" back arrow on History. */
@Composable
fun CircleIconButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    Box(
        modifier = modifier
            .size(32.dp)
            .clip(CircleShape)
            .background(Color.White.copy(alpha = 0.04f))
            .border(1.dp, AppOutline, CircleShape)
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Text(text = label, color = AppOnBackground, fontSize = 15.sp)
    }
}

/** Small rounded status pill, e.g. "Finalizada" / "En curso" on history cards. */
@Composable
fun StatusPill(
    text: String,
    containerColor: Color,
    contentColor: Color,
    modifier: Modifier = Modifier
) {
    Box(
        modifier = modifier
            .clip(PillShape)
            .background(containerColor)
            .padding(horizontal = 10.dp, vertical = 3.dp)
    ) {
        Text(text = text, color = contentColor, fontWeight = FontWeight.SemiBold, fontSize = 11.sp)
    }
}

/**
 * Full-screen dark background with two soft violet glow orbs, matching the portrait-flow
 * screens in the mockup (Login, Dashboard, History, Setup).
 */
@Composable
fun AppScreenBackground(
    modifier: Modifier = Modifier,
    content: @Composable BoxScope.() -> Unit
) {
    Box(modifier = modifier.fillMaxSize().background(AppBackground)) {
        Box(
            modifier = Modifier
                .align(Alignment.TopEnd)
                .offset(x = 100.dp, y = (-100).dp)
                .size(280.dp)
                .background(
                    Brush.radialGradient(
                        listOf(Color(0x52A78BFA), Color(0x00A78BFA))
                    ),
                    CircleShape
                )
        )
        Box(
            modifier = Modifier
                .align(Alignment.BottomStart)
                .offset(x = (-100).dp, y = 120.dp)
                .size(300.dp)
                .background(
                    Brush.radialGradient(
                        listOf(Color(0x33A855F7), Color(0x00A855F7))
                    ),
                    CircleShape
                )
        )
        content()
    }
}

/**
 * Prompt shown on the landscape-only screens (Pregame, GameTracker) while the device is still
 * in portrait — these screens read real orientation via `LocalConfiguration`, no fake timers.
 */
@Composable
fun RotateDevicePrompt(message: String, modifier: Modifier = Modifier) {
    Column(
        modifier = modifier.fillMaxSize(),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Box(
            modifier = Modifier
                .size(52.dp, 92.dp)
                .clip(RoundedCornerShape(10.dp))
                .border(3.dp, AccentSoft, RoundedCornerShape(10.dp))
        )
        Spacer(modifier = Modifier.height(22.dp))
        Text(
            text = message,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            fontSize = 13.sp,
            textAlign = TextAlign.Center
        )
    }
}

/** Thin translucent divider used to split a form section, e.g. "email o Google". */
@Composable
fun ThinDivider(modifier: Modifier = Modifier) {
    Box(
        modifier = modifier
            .height(1.dp)
            .background(AppOutline)
    )
}

/** Labeled pill-shaped text field shared by Login/Register forms. */
@Composable
fun AuthTextField(
    label: String,
    value: String,
    onValueChange: (String) -> Unit,
    enabled: Boolean,
    modifier: Modifier = Modifier,
    keyboardType: KeyboardType = KeyboardType.Text,
    visualTransformation: VisualTransformation = VisualTransformation.None
) {
    Column(modifier = modifier.fillMaxWidth()) {
        Text(text = label, fontSize = 12.sp, color = AppFaint)
        Spacer(modifier = Modifier.height(6.dp))
        OutlinedTextField(
            value = value,
            onValueChange = onValueChange,
            enabled = enabled,
            singleLine = true,
            visualTransformation = visualTransformation,
            keyboardOptions = KeyboardOptions(keyboardType = keyboardType),
            shape = PillShape,
            colors = OutlinedTextFieldDefaults.colors(
                unfocusedContainerColor = MaterialTheme.colorScheme.background.copy(alpha = 0.3f),
                focusedContainerColor = MaterialTheme.colorScheme.background.copy(alpha = 0.3f),
                unfocusedBorderColor = AppOutline,
                focusedBorderColor = MaterialTheme.colorScheme.primary,
                unfocusedTextColor = MaterialTheme.colorScheme.onBackground,
                focusedTextColor = MaterialTheme.colorScheme.onBackground
            ),
            modifier = Modifier.fillMaxWidth()
        )
    }
}

/** Uppercase eyebrow label ("Login", "Nueva partida", ...). */
@Composable
fun SectionEyebrow(text: String, modifier: Modifier = Modifier) {
    Text(
        text = text.uppercase(),
        modifier = modifier,
        color = AppFaint,
        fontWeight = FontWeight.SemiBold,
        fontSize = 12.sp,
        letterSpacing = 1.sp
    )
}
