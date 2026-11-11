import QtQuick
import QtQuick.Templates as T

T.ProgressBar {
    id: control

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            implicitContentWidth + leftPadding + rightPadding)
    implicitHeight: Math.max(implicitBackgroundHeight + topInset + bottomInset,
                             implicitContentHeight + topPadding + bottomPadding)

    topPadding: painter.paddingTop
    bottomPadding: painter.paddingBottom

    contentItem: Item {
        implicitHeight: metrics.height
    }

    FontMetrics {
        id: metrics
        font: control.font
    }

    background: StyleItem {
        id: painter
        element: StyleItem.ProgressBar
        value: control.indeterminate ? -1 : control.position
        font: control.font
    }
}
