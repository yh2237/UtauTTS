pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// ライセンス表示の共有ビュー。ウィンドウ装飾は shell 側が担当する。
RowLayout {
    id: content
    property var documents: []

    anchors.margins: 10
    spacing: 8

    ListView {
        id: licenseList
        Layout.preferredWidth: 210
        Layout.fillHeight: true
        clip: true
        model: content.documents
        currentIndex: 0

        delegate: ItemDelegate {
            required property int index
            required property var modelData
            width: ListView.view.width
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
