package com.commandercompanion.presentation.navigation

import java.net.URLDecoder
import java.net.URLEncoder

/**
 * A tracker seat. In Casual mode, [assignedUserId]/[assignedUsername]/[deckId] are
 * always null (no remote identity, no sync — see [PlaygroupRepository]). In Group mode,
 * a seat can be assigned to a real playgroup member ([assignedUserId] + a chosen
 * [deckId]) or stay a "guest" (same fields null, just like Casual). There's no need to
 * know "which seat am I": [GameRepository.bootstrapRemoteGame] passes each assigned seat's
 * [assignedUserId] as-is to `join` — the backend decides whether it's a self-join or a
 * proxy-join by comparing it against the authenticated user.
 */
data class PlayerConfig(
    val name: String,
    val colorKey: String,
    val mulligans: Int = 0,
    val assignedUserId: String? = null,
    val assignedUsername: String? = null,
    val deckId: String? = null
)

private const val ENTRY_SEPARATOR = ","
private const val FIELD_SEPARATOR = "|"
private const val CHARSET = "UTF-8"

fun encodePlayerConfigs(configs: List<PlayerConfig>): String =
    configs.joinToString(ENTRY_SEPARATOR) { config ->
        val userIdField = config.assignedUserId?.let { URLEncoder.encode(it, CHARSET) } ?: ""
        val usernameField = config.assignedUsername?.let { URLEncoder.encode(it, CHARSET) } ?: ""
        val deckField = config.deckId?.let { URLEncoder.encode(it, CHARSET) } ?: ""
        "${URLEncoder.encode(config.name, CHARSET)}$FIELD_SEPARATOR${config.colorKey}$FIELD_SEPARATOR" +
            "${config.mulligans}$FIELD_SEPARATOR$userIdField$FIELD_SEPARATOR$usernameField$FIELD_SEPARATOR$deckField"
    }

fun decodePlayerConfigs(encoded: String): List<PlayerConfig> =
    encoded.split(ENTRY_SEPARATOR).filter { it.isNotBlank() }.map { entry ->
        val parts = entry.split(FIELD_SEPARATOR, limit = 6)
        PlayerConfig(
            name = URLDecoder.decode(parts[0], CHARSET),
            colorKey = parts[1],
            mulligans = parts.getOrNull(2)?.toIntOrNull() ?: 0,
            assignedUserId = parts.getOrNull(3)?.takeIf { it.isNotEmpty() }?.let { URLDecoder.decode(it, CHARSET) },
            assignedUsername = parts.getOrNull(4)?.takeIf { it.isNotEmpty() }?.let { URLDecoder.decode(it, CHARSET) },
            deckId = parts.getOrNull(5)?.takeIf { it.isNotEmpty() }?.let { URLDecoder.decode(it, CHARSET) }
        )
    }
