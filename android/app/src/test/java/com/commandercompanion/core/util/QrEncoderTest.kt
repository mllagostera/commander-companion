package com.commandercompanion.core.util

import com.google.zxing.BinaryBitmap
import com.google.zxing.LuminanceSource
import com.google.zxing.common.BitMatrix
import com.google.zxing.common.HybridBinarizer
import com.google.zxing.qrcode.QRCodeReader
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

private const val USER_ID = "0d2dff0e-6208-4783-8236-e83974d900c6"

private const val BLACK: Byte = 0
private const val WHITE: Byte = -1 // 255 unsigned

/**
 * Reads a [BitMatrix] as greyscale so the encoded QR can be decoded again in
 * the same test. ZXing ships one of these for `BufferedImage`, but that lives
 * in the `javase` artifact; this is the handful of lines needed to avoid
 * pulling in a second dependency just for tests.
 */
private class BitMatrixLuminanceSource(private val matrix: BitMatrix) :
    LuminanceSource(matrix.width, matrix.height) {

    override fun getRow(y: Int, row: ByteArray?): ByteArray {
        val out = if (row != null && row.size >= width) row else ByteArray(width)
        for (x in 0 until width) out[x] = if (matrix.get(x, y)) BLACK else WHITE
        return out
    }

    override fun getMatrix(): ByteArray {
        val out = ByteArray(width * height)
        for (y in 0 until height) {
            for (x in 0 until width) out[y * width + x] = if (matrix.get(x, y)) BLACK else WHITE
        }
        return out
    }
}

private fun decode(matrix: BitMatrix): String =
    QRCodeReader().decode(BinaryBitmap(HybridBinarizer(BitMatrixLuminanceSource(matrix)))).text

class QrEncoderTest {

    /**
     * The strongest check available without a camera: encode the link, decode
     * the resulting matrix, and assert the text survived the round trip. A
     * generated QR that renders but decodes to the wrong thing would otherwise
     * only be caught on a real phone.
     */
    @Test
    fun `el QR generado se decodifica al mismo enlace`() {
        val link = friendQrLink("https://commander.example", USER_ID)

        assertEquals(link, decode(QrEncoder.encode(link)))
    }

    @Test
    fun `el enlace tiene la misma forma que el del cliente web`() {
        assertEquals(
            "https://commander.example/friends/add/$USER_ID",
            friendQrLink("https://commander.example", USER_ID)
        )
    }

    /** BuildConfig values are easy to write with a trailing slash. */
    @Test
    fun `una barra final en la URL base no duplica la barra`() {
        assertEquals(
            "https://commander.example/friends/add/$USER_ID",
            friendQrLink("https://commander.example/", USER_ID)
        )
    }

    @Test
    fun `el QR es cuadrado y no esta vacio`() {
        val matrix = QrEncoder.encode(friendQrLink("https://commander.example", USER_ID))

        assertEquals(matrix.width, matrix.height)
        assertTrue(matrix.width > 0)
    }

    // ------------------------------------------------------------- parsing

    @Test
    fun `un enlace escaneado devuelve el id`() {
        assertEquals(USER_ID, parseScannedFriendCode("https://commander.example/friends/add/$USER_ID"))
    }

    /**
     * The web client encoded a bare UUID before the deep link existed, and a
     * screenshot of one of those is still a valid way to be added.
     */
    @Test
    fun `un uuid pelado sigue siendo valido`() {
        assertEquals(USER_ID, parseScannedFriendCode(USER_ID))
    }

    @Test
    fun `se ignoran query y fragmento`() {
        assertEquals(USER_ID, parseScannedFriendCode("https://commander.example/friends/add/$USER_ID?utm=qr"))
        assertEquals(USER_ID, parseScannedFriendCode("https://commander.example/friends/add/$USER_ID#top"))
    }

    @Test
    fun `se recortan los espacios`() {
        assertEquals(USER_ID, parseScannedFriendCode("  $USER_ID  "))
    }

    /**
     * A camera pointed at the world reads all sorts of codes. None of them
     * should reach the backend.
     */
    @Test
    fun `cualquier otro codigo se rechaza`() {
        val rejected = listOf(
            null,
            "",
            "   ",
            "https://example.com/",
            "WIFI:S:MiRed;T:WPA;P:secreto;;",
            "8412345678905",
            "https://commander.example/friends/add/no-es-un-uuid",
            "https://commander.example/friends/add/",
            "0d2dff0e62084783823 6e83974d900c6"
        )

        rejected.forEach { assertNull("debería rechazar: $it", parseScannedFriendCode(it)) }
    }
}
