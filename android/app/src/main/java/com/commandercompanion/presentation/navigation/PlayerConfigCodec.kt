package com.commandercompanion.presentation.navigation

import java.net.URLDecoder
import java.net.URLEncoder

data class PlayerConfig(val name: String, val colorKey: String)

private const val ENTRY_SEPARATOR = ","
private const val FIELD_SEPARATOR = "|"
private const val CHARSET = "UTF-8"

fun encodePlayerConfigs(configs: List<PlayerConfig>): String =
    configs.joinToString(ENTRY_SEPARATOR) { config ->
        "${URLEncoder.encode(config.name, CHARSET)}$FIELD_SEPARATOR${config.colorKey}"
    }

fun decodePlayerConfigs(encoded: String): List<PlayerConfig> =
    encoded.split(ENTRY_SEPARATOR).filter { it.isNotBlank() }.map { entry ->
        val (name, colorKey) = entry.split(FIELD_SEPARATOR, limit = 2)
        PlayerConfig(name = URLDecoder.decode(name, CHARSET), colorKey = colorKey)
    }
