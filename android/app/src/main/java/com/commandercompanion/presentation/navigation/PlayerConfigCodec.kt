package com.commandercompanion.presentation.navigation

import java.net.URLDecoder
import java.net.URLEncoder

/**
 * Un asiento del tracker. En modo Casual, [assignedUserId]/[assignedUsername]/[deckId] son
 * siempre null (sin identidad remota, sin sync — ver [PlaygroupRepository]). En modo Grupo,
 * un asiento puede quedar asignado a un miembro real del playgroup ([assignedUserId] +
 * [deckId] elegido) o quedar como "invitado" (mismos campos null, igual que Casual). No hace
 * falta saber "cuál asiento soy yo": [GameRepository.bootstrapRemoteGame] pasa el
 * [assignedUserId] de cada asiento asignado tal cual al `join` — el backend decide si es un
 * self-join o un proxy-join comparándolo contra el usuario autenticado.
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
