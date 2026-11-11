import QtQuick
import QtQuick.Templates as T
import QtQuick.Controls.impl

T.TextArea {
    id: control

    implicitWidth: Math.max(contentWidth + leftPadding + rightPadding,
                            implicitBackgroundWidth + leftInset + rightInset,
                            placeholder.implicitWidth + leftPadding + rightPadding)
    implicitHeight: Math.max(contentHeight + topPadding + bottomPadding,
                             implicitBackgroundHeight + topInset + bottomInset,
                             placeholder.implicitHeight + topPadding + bottomPadding)

    // QTextEdit と同じく、枠の内側に文書の余白（QTextDocument の既定値4）を空ける。
    padding: painter.pixelMetric(StyleItem.DefaultFrameWidth) + 4
    color: control.palette.text
    selectionColor: control.palette.highlight
    selectedTextColor: control.palette.highlightedText
    placeholderTextColor: control.palette.placeholderText

    PlaceholderText {
        id: placeholder
        x: control.leftPadding
        y: control.topPadding
        width: control.width - (control.leftPadding + control.rightPadding)
        height: control.height - (control.topPadding + control.bottomPadding)
        text: control.placeholderText
        font: control.font
        color: control.placeholderTextColor
        verticalAlignment: control.verticalAlignment
        visible: !control.length && !control.preeditText && (!control.activeFocus || control.horizontalAlignment !== Qt.AlignHCenter)
        elide: Text.ElideRight
        renderType: control.renderType
    }

    // ScrollView に入れたときは、枠を ScrollView 側に任せて二重にしない。
    background: StyleItem {
        id: painter
        visible: !(control.parent instanceof Flickable)
        element: StyleItem.TextField
        hovered: control.hovered
        focused: control.activeFocus
        editable: !control.readOnly
        font: control.font
    }
}
