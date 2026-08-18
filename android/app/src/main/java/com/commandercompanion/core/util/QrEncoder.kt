package com.commandercompanion.core.util

import com.google.zxing.BarcodeFormat
import com.google.zxing.EncodeHintType
import com.google.zxing.common.BitMatrix
import com.google.zxing.qrcode.QRCodeWriter
import com.google.zxing.qrcode.decoder.ErrorCorrectionLevel

/**
 * Profile-QR encoding, kept free of any Android type on purpose.
 *
 * ZXing's `core` module is pure Java, so everything here runs (and is tested)
 * on the JVM. Turning the [BitMatrix] into something drawable needs
 * `android.graphics.Bitmap`, which does not exist in a unit test — that step
 * lives in the UI layer instead, so the part with actual logic stays testable.
 */
object QrEncoder {

    /** Size of the generated matrix, in modules-with-quiet-zone. */
    private const val DEFAULT_SIZE = 512

    /** Matches the web client's `margin: 1` (see settings.vue). */
    private const val QUIET_ZONE_MODULES = 1

    fun encode(content: String, size: Int = DEFAULT_SIZE): BitMatrix =
        QRCodeWriter().encode(
            content,
            BarcodeFormat.QR_CODE,
            size,
            size,
            mapOf(
                EncodeHintType.MARGIN to QUIET_ZONE_MODULES,
                // A phone screen is scanned close up and clean, so the lowest
                // correction level is enough and keeps the modules large,
                // which is what actually helps a camera lock on.
                EncodeHintType.ERROR_CORRECTION to ErrorCorrectionLevel.L,
                EncodeHintType.CHARACTER_SET to "UTF-8"
            )
        )
}

/**
 * The link a profile QR encodes.
 *
 * Must stay byte-identical to what the web client builds (see
 * `web/app/pages/settings.vue`), because either client has to be able to scan
 * the other's code — and it is the URL the Android App Link filter matches.
 * [webAppUrl] comes from `BuildConfig.WEB_APP_URL`.
 */
fun friendQrLink(webAppUrl: String, userId: String): String =
    "${webAppUrl.trimEnd('/')}/friends/add/$userId"

/** Path prefix of [friendQrLink], shared with whatever parses a scan back. */
const val FRIEND_LINK_PATH_PREFIX = "/friends/add/"

private val UUID_REGEX =
    Regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

/**
 * Pulls the user id out of a scanned code, or null if it isn't one of ours.
 *
 * Accepts both the deep link and a bare UUID: the bare form is what the web
 * client encoded before the link existed, and a QR someone screenshotted then
 * is still a perfectly good way to be added.
 *
 * Anything else — a Wi-Fi QR, a product barcode, a URL from another site — is
 * rejected here rather than sent to the backend to be rejected there.
 */
fun parseScannedFriendCode(raw: String?): String? {
    val text = raw?.trim().orEmpty()
    if (text.isEmpty()) return null

    val candidate = when {
        text.contains(FRIEND_LINK_PATH_PREFIX) ->
            text.substringAfterLast(FRIEND_LINK_PATH_PREFIX).substringBefore('?').substringBefore('#')
        else -> text
    }
    return candidate.takeIf { UUID_REGEX.matches(it) }
}
