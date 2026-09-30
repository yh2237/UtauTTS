pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

GridLayout {
    id: content
    property var documents: []
    property int currentIndex: 0
    readonly property bool compact: width < 600
    readonly property var currentDocument: currentIndex >= 0 && currentIndex < documents.length
            ? documents[currentIndex] : null
    onDocumentsChanged: currentIndex = documents.length ? Math.max(0, Math.min(currentIndex, documents.length - 1)) : -1

    anchors.margins: 10
    columns: compact ? 1 : 2
    columnSpacing: 8
    rowSpacing: 8

    ColumnLayout {
        visible: content.compact
        Layout.fillWidth: true
        spacing: 12
        ComboBox {
            id: documentSelector
            Layout.fillWidth: true
            Layout.preferredHeight: 48
            model: content.documents
            textRole: "name"
            currentIndex: content.currentIndex
            enabled: count > 0
            onActivated: {
                content.currentIndex = currentIndex;
                documentScroll.contentItem.contentY = 0;
            }
            delegate: ItemDelegate {
                required property int index
                required property var modelData
                width: documentSelector.width
                implicitHeight: Math.max(48, documentName.implicitHeight + topPadding + bottomPadding)
                highlighted: documentSelector.highlightedIndex === index
                contentItem: Text {
                    id: documentName
                    text: modelData.name
                    font: documentSelector.font
                    color: palette.text
                    wrapMode: Text.Wrap
                    verticalAlignment: Text.AlignVCenter
                }
            }
        }
        Label {
            Layout.fillWidth: true
            text: content.currentDocument ? content.currentDocument.name : ""
            font.bold: true
            wrapMode: Text.Wrap
        }
    }

    ListView {
        id: licenseList
        visible: !content.compact
        Layout.preferredWidth: 210
        Layout.fillHeight: true
        clip: true
        model: content.documents
        currentIndex: content.currentIndex
        onCurrentIndexChanged: {
            if (currentIndex >= 0 && currentIndex !== content.currentIndex)
                content.currentIndex = currentIndex;
        }

        delegate: ItemDelegate {
            required property int index
            required property var modelData
            width: ListView.view.width
            text: modelData.name
            highlighted: ListView.isCurrentItem
            onClicked: content.currentIndex = index
        }
    }

    ScrollView {
        id: documentScroll
        Layout.fillWidth: true
        Layout.fillHeight: true
        contentWidth: availableWidth
        ScrollBar.horizontal.policy: ScrollBar.AlwaysOff
        TextArea {
            width: documentScroll.availableWidth
            text: content.currentDocument ? content.currentDocument.text : ""
            textFormat: TextEdit.PlainText
            font: content.compact ? Qt.font({family: Qt.application.font.family, pixelSize: 14}) : Qt.application.font
            leftPadding: content.compact ? 0 : 6
            rightPadding: content.compact ? 0 : 6
            readOnly: true
            selectByMouse: true
            wrapMode: TextEdit.Wrap
        }
    }
}
