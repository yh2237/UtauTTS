pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls

ListFrame {
    id: root
    property bool compact: false
    property alias model: list.model
    property alias currentIndex: list.currentIndex
    readonly property alias count: list.count
    property string textRole: ""
    signal clicked(int index)

    framed: !compact

    ListView {
        id: list
        anchors.fill: parent
        orientation: root.compact ? ListView.Horizontal : ListView.Vertical
        clip: true
        ScrollBar.vertical: ScrollBar {}

        delegate: ItemDelegate {
            required property int index
            required property var modelData
            width: root.compact ? Math.max(Theme.fieldWidthSmall, implicitWidth) : ListView.view.width
            leftPadding: Theme.spacing
            rightPadding: Theme.spacing
            topPadding: Theme.spacingMedium
            bottomPadding: Theme.spacingMedium
            text: root.textRole ? modelData[root.textRole] : modelData
            highlighted: ListView.isCurrentItem
            onClicked: root.clicked(index)
        }
    }
}
