package dev.yuva.ui

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.PathBuilder
import androidx.compose.ui.graphics.vector.path
import androidx.compose.ui.unit.dp

internal object YuvaIcons {
    val Bug: ImageVector by lazy {
        outline("Bug") {
            moveTo(8.5f, 9.5f)
            horizontalLineToRelative(7f)
            verticalLineToRelative(5.5f)
            arcToRelative(3.5f, 3.5f, 0f, false, true, -7f, 0f)
            close()
            moveTo(9.5f, 8f)
            arcToRelative(2.5f, 2.5f, 0f, false, true, 5f, 0f)
            moveTo(12f, 9.5f)
            verticalLineToRelative(9f)
            moveTo(8.5f, 11.5f)
            lineTo(5f, 10.5f)
            moveTo(8.5f, 16f)
            lineTo(5f, 17.5f)
            moveTo(15.5f, 11.5f)
            lineTo(19f, 10.5f)
            moveTo(15.5f, 16f)
            lineTo(19f, 17.5f)
            moveTo(10.2f, 5.7f)
            lineTo(8.7f, 4f)
            moveTo(13.8f, 5.7f)
            lineTo(15.3f, 4f)
        }
    }

    val Lightbulb: ImageVector by lazy {
        outline("Lightbulb") {
            moveTo(9.2f, 15.5f)
            curveTo(9.2f, 14.5f, 8.5f, 14f, 8f, 13.3f)
            arcTo(5f, 5f, 0f, true, true, 16f, 13.3f)
            curveTo(15.5f, 14f, 14.8f, 14.5f, 14.8f, 15.5f)
            close()
            moveTo(9.5f, 18.3f)
            horizontalLineToRelative(5f)
            moveTo(10.5f, 20.8f)
            horizontalLineToRelative(3f)
        }
    }

    val Chat: ImageVector by lazy {
        outline("Chat") {
            moveTo(6f, 4.5f)
            horizontalLineToRelative(12f)
            arcToRelative(2.5f, 2.5f, 0f, false, true, 2.5f, 2.5f)
            verticalLineToRelative(7f)
            arcToRelative(2.5f, 2.5f, 0f, false, true, -2.5f, 2.5f)
            horizontalLineToRelative(-6.5f)
            lineTo(7.5f, 20f)
            verticalLineToRelative(-3.5f)
            horizontalLineTo(6f)
            arcTo(2.5f, 2.5f, 0f, false, true, 3.5f, 14f)
            verticalLineTo(7f)
            arcTo(2.5f, 2.5f, 0f, false, true, 6f, 4.5f)
            close()
        }
    }

    val Attach: ImageVector by lazy {
        outline("Attach") {
            moveTo(16.5f, 7.5f)
            verticalLineToRelative(8.2f)
            arcToRelative(4.5f, 4.5f, 0f, false, true, -9f, 0f)
            verticalLineTo(6.5f)
            arcToRelative(3f, 3f, 0f, false, true, 6f, 0f)
            verticalLineToRelative(8.7f)
            arcToRelative(1.5f, 1.5f, 0f, false, true, -3f, 0f)
            verticalLineTo(8.5f)
        }
    }

    private fun outline(name: String, block: PathBuilder.() -> Unit): ImageVector =
        ImageVector.Builder(name, 24.dp, 24.dp, 24f, 24f).apply {
            path(
                stroke = SolidColor(Color.Black),
                strokeLineWidth = 1.8f,
                strokeLineCap = StrokeCap.Round,
                strokeLineJoin = StrokeJoin.Round,
                pathBuilder = block,
            )
        }.build()
}
