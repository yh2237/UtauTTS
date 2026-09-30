pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls

// Fusionの辺別余白が優先されるため、paddingだけでなく各辺を指定する。
Dialog {
    id: root
    readonly property int pageMargin: 16
    parent: Overlay.overlay
    modal: true
    width: parent ? parent.width : 0
    height: parent ? parent.height : 0
    leftPadding: pageMargin
    rightPadding: pageMargin
    topPadding: pageMargin
    bottomPadding: pageMargin
    exit: Transition {}
    header: WindowHeader {
        leftPadding: 0
        rightPadding: 0
        heading: root.title
        contentMargin: root.pageMargin
        closeButtonMargin: root.pageMargin
        onCloseClicked: root.close()
    }
}
