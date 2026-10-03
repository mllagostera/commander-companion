package com.vansid.tapeandocartones.presentation.screens.dashboard

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.lifecycle.compose.LifecycleResumeEffect
import com.vansid.tapeandocartones.R
import com.vansid.tapeandocartones.presentation.components.AppLogoMark
import com.vansid.tapeandocartones.presentation.components.AppScreenBackground
import com.vansid.tapeandocartones.presentation.components.GradientButton
import com.vansid.tapeandocartones.presentation.components.GradientOutlineButton
import com.vansid.tapeandocartones.presentation.components.GradientTitle
import com.vansid.tapeandocartones.presentation.theme.AppFaint
import com.vansid.tapeandocartones.presentation.theme.TitleGradient
import com.vansid.tapeandocartones.presentation.theme.StatusDanger

@Composable
fun DashboardScreen(
    onNewGame: () -> Unit,
    onJoinGame: () -> Unit,
    onViewHistory: () -> Unit,
    onViewStatistics: () -> Unit,
    onViewFriends: () -> Unit,
    onOpenSettings: () -> Unit,
    onLogout: () -> Unit,
    viewModel: DashboardViewModel = hiltViewModel()
) {
    val pendingFriendRequests by viewModel.pendingFriendRequests.collectAsState()

    // On every resume, not just the first composition: coming back from the friends screen or
    // from the background is exactly when the count may have changed.
    LifecycleResumeEffect(Unit) {
        viewModel.refreshPendingFriendRequests()
        onPauseOrDispose {}
    }

    AppScreenBackground {
        Column(
            modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            AppLogoMark(width = 42.dp, height = 58.dp)
            Spacer(modifier = Modifier.height(20.dp))
            GradientTitle(text = stringResource(R.string.app_name), fontSize = 26.sp)
            Spacer(modifier = Modifier.height(24.dp))

            GradientButton(text = stringResource(R.string.dashboard_new_game), onClick = onNewGame) {
                Text(
                    stringResource(R.string.dashboard_new_game),
                    color = MaterialTheme.colorScheme.background,
                    fontWeight = FontWeight.Bold,
                    fontSize = 15.sp,
                    letterSpacing = 0.5.sp
                )
            }
            Spacer(modifier = Modifier.height(14.dp))
            GradientOutlineButton(text = stringResource(R.string.dashboard_join_game), onClick = onJoinGame)
            Spacer(modifier = Modifier.height(14.dp))
            GradientOutlineButton(text = stringResource(R.string.dashboard_history), onClick = onViewHistory)
            Spacer(modifier = Modifier.height(20.dp))
            Text(
                text = stringResource(R.string.dashboard_statistics),
                color = AppFaint,
                fontSize = 13.sp,
                modifier = Modifier.clickable(onClick = onViewStatistics)
            )
            Spacer(modifier = Modifier.height(10.dp))
            FriendsEntry(pendingRequests = pendingFriendRequests, onClick = onViewFriends)
            Spacer(modifier = Modifier.height(10.dp))
            Text(
                text = stringResource(R.string.dashboard_settings),
                color = AppFaint,
                fontSize = 13.sp,
                modifier = Modifier.clickable(onClick = onOpenSettings)
            )
            Spacer(modifier = Modifier.height(10.dp))
            Text(
                text = stringResource(R.string.dashboard_logout),
                color = StatusDanger,
                fontSize = 13.sp,
                modifier = Modifier.clickable { viewModel.logout(onLogout) }
            )
        }
    }
}

/**
 * The "Friends" link plus, when there are pending requests, a badge with how many. Screen
 * readers get one combined label ("Friends, 2 pending friend requests") instead of a bare number.
 */
@Composable
private fun FriendsEntry(pendingRequests: Int, onClick: () -> Unit) {
    val label = stringResource(R.string.dashboard_friends)
    val description = if (pendingRequests > 0) {
        "$label, " + pluralStringResource(
            R.plurals.dashboard_pending_friend_requests, pendingRequests, pendingRequests
        )
    } else {
        label
    }
    Row(
        verticalAlignment = Alignment.CenterVertically,
        modifier = Modifier
            .clickable(onClick = onClick)
            .clearAndSetSemantics { contentDescription = description }
    ) {
        Text(text = label, color = AppFaint, fontSize = 13.sp)
        if (pendingRequests > 0) {
            Spacer(modifier = Modifier.width(6.dp))
            Box(
                contentAlignment = Alignment.Center,
                modifier = Modifier
                    .defaultMinSize(minWidth = 18.dp, minHeight = 18.dp)
                    .background(TitleGradient, CircleShape)
                    .padding(horizontal = 5.dp)
            ) {
                Text(
                    // Past 9 the exact number doesn't change what the user does.
                    text = if (pendingRequests > 9) "9+" else pendingRequests.toString(),
                    color = MaterialTheme.colorScheme.background,
                    fontSize = 11.sp,
                    fontWeight = FontWeight.Bold
                )
            }
        }
    }
}
