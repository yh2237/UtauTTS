pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// ライセンス表示の共有ビュー。ウィンドウ装飾は shell 側が担当する。
GridLayout {
    id: content
    property var documents: []
    readonly property bool compact: width < 600

    anchors.margins: 10
    columns: compact ? 1 : 2
    columnSpacing: 8
    rowSpacing: 8

    ListView {
        id: licenseList
        Layout.preferredWidth: content.compact ? -1 : 210
        Layout.preferredHeight: content.compact ? 48 : -1
        Layout.fillWidth: content.compact
        Layout.fillHeight: !content.compact
        orientation: content.compact ? ListView.Horizontal : ListView.Vertical
        clip: true
        model: content.documents
        currentIndex: 0

        delegate: ItemDelegate {
            required property int index
            required property var modelData
            width: content.compact ? Math.max(120, implicitWidth) : ListView.view.width
            text: modelData.name
            highlighted: ListView.isCurrentItem
            onClicked: licenseList.currentIndex = index
        }
    }

    ScrollView {
        Layout.fillWidth: true
        Layout.fillHeight: true
        TextArea {
            width: parent.width
            text: content.documents.length && licenseList.currentIndex >= 0
                  ? content.documents[licenseList.currentIndex].text : ""
            readOnly: true
            selectByMouse: true
            wrapMode: TextEdit.Wrap
        }
    }
}
