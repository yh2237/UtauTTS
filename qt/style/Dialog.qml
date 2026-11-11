import QtQuick
import QtQuick.Templates as T
import QtQuick.Controls.impl

T.Dialog {
    id: control

    readonly property int windowMargin: metrics.pixelMetric(StyleItem.WindowMargin)
    readonly property int layoutSpacing: metrics.pixelMetric(StyleItem.LayoutSpacing)

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            contentWidth + leftPadding + rightPadding,
                            implicitHeaderWidth,
                            implicitFooterWidth)
    implicitHeight: Math.max(implicitBackgroundHeight + topInset + bottomInset,
                             contentHeight + topPadding + bottomPadding
                             + (implicitHeaderHeight > 0 ? implicitHeaderHeight + spacing : 0)
                             + (implicitFooterHeight > 0 ? implicitFooterHeight + spacing : 0))

    spacing: 0
    leftPadding: windowMargin
    rightPadding: windowMargin
    topPadding: header && header.visible ? layoutSpacing : windowMargin
    bottomPadding: footer && footer.visible ? layoutSpacing : windowMargin

    background: Rectangle {
        color: control.palette.window
        border.color: control.palette.dark

        StyleItem {
            id: metrics
            visible: false
            element: StyleItem.Frame
        }
    }

    header: T.Label {
        text: control.title
        visible: control.title.length > 0 && parent?.parent === T.Overlay.overlay
        elide: Text.ElideRight
        font.bold: true
        color: control.palette.windowText
        leftPadding: control.windowMargin
        rightPadding: control.windowMargin
        topPadding: control.windowMargin
    }

    footer: DialogButtonBox {
        visible: count > 0
        leftPadding: control.windowMargin
        rightPadding: control.windowMargin
        bottomPadding: control.windowMargin
    }

    T.Overlay.modal: Rectangle {
        color: Color.transparent(control.palette.shadow, 0.4)
    }

    T.Overlay.modeless: Rectangle {
        color: Color.transparent(control.palette.shadow, 0.4)
    }
}
