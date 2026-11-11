import QtQuick
import QtQuick.Templates as T

T.DialogButtonBox {
    id: control

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            contentWidth + leftPadding + rightPadding)
    implicitHeight: Math.max(implicitBackgroundHeight + topInset + bottomInset,
                             contentHeight + topPadding + bottomPadding)

    spacing: metrics.pixelMetric(StyleItem.LayoutSpacing)
    padding: 0
    alignment: Qt.AlignRight

    delegate: Button {}

    contentItem: ListView {
        implicitWidth: contentWidth
        implicitHeight: heightProbe.implicitHeight
        model: control.contentModel
        spacing: control.spacing
        orientation: ListView.Horizontal
        boundsBehavior: Flickable.StopAtBounds
        snapMode: ListView.SnapToItem
    }

    // 並んだボタンから高さを取るとループになるので、同じ字のボタンで測る。
    background: Item {
        StyleItem {
            id: metrics
            visible: false
            element: StyleItem.Frame
        }
        Button {
            id: heightProbe
            visible: false
            text: "Ag"
            font: control.font
        }
    }
}
