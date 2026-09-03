package com.commandercompanion.presentation.screens.setup

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.domain.model.Playgroup
import com.commandercompanion.domain.repository.PlaygroupRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.launch

/**
 * Data for [PlayerSetupScreen]'s Group mode: the authenticated user's playgroups, to pick which
 * one this game belongs to. Best-effort: if it fails (no network, no session) Group mode is left
 * with no groups to choose from — it doesn't block Casual mode.
 *
 * Seat/member/deck assignment itself happens later, on
 * [com.commandercompanion.presentation.screens.pregame.PreGameScreen] (see
 * `PreGameViewModel`) — not here.
 */
@HiltViewModel
class PlayerSetupViewModel @Inject constructor(
    private val playgroupRepository: PlaygroupRepository
) : ViewModel() {

    var playgroups by mutableStateOf<List<Playgroup>>(emptyList())
        private set

    init {
        viewModelScope.launch {
            playgroupRepository.listPlaygroups().onSuccess { playgroups = it }
        }
    }
}
