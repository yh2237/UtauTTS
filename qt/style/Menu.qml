import QtQuick
import QtQuick.Window
import QtQuick.Templates as T

T.Menu {
    id: control

    readonly property real itemsWidth: {
        let widest = 0;
        for (let index = 0; index < count; ++index) {
            const item = itemAt(index);
            if (item)
                widest = Math.max(widest, item.implicitWidth);
        }
        return widest;
    }

    implicitWidth: Math.max(implicitBackgroundWidth + leftInset + rightInset,
                            itemsWidth + leftPadding + rightPadding)
    implicitHeight: Math.max(implicitBackgroundHeight + topInset + bottomInset,
                             contentHeight + topPadding + bottomPadding)

    margins: 0
    leftPadding: painter.paddingLeft
    topPadding: painter.paddingTop
    rightPadding: painter.paddingRight
    bottomPadding: painter.paddingBottom
    overlap: painter.paddingLeft

    delegate: MenuItem {}

    contentItem: ListView {
        implicitHeight: contentHeight
        model: control.contentModel
        interactive: Window.window
                     ? contentHeight + control.topPadding + control.bottomPadding > control.height
                     : false
        clip: true
        currentIndex: control.currentIndex
        T.ScrollBar.vertical: ScrollBar {}
    }

    background: StyleItem {
        id: painter
        element: StyleItem.MenuPanel
    }
}
