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

    anchors.margins: Theme.spacingLarge
    columns: compact ? 1 : 2
    columnSpacing: Theme.spacingLarge
    rowSpacing: Theme.spacingLarge

    ColumnLayout {
        visible: content.compact
        Layout.fillWidth: true
        spacing: Theme.spacingLarge
        ComboBox {
            id: documentSelector
            Layout.fillWidth: true
            Layout.preferredHeight: Theme.touchTarget
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
                implicitHeight: Math.max(Theme.touchTarget, documentName.implicitHeight + topPadding + bottomPadding)
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

    SideList {
        id: licenseList
        visible: !content.compact
        Layout.preferredWidth: Theme.sidebarWidth
        Layout.fillHeight: true
        model: content.documents
        textRole: "name"
        currentIndex: content.currentIndex
        onClicked: index => content.currentIndex = index
    }

    ScrollView {
        id: documentScroll
        Layout.fillWidth: true
        Layout.fillHeight: true
        contentWidth: availableWidth
        ScrollBar.horizontal.policy: ScrollBar.AlwaysOff
        // 出入りで幅が変わると折り返しと高さが循環するので、縦のスクロールバーは常に出す。
        ScrollBar.vertical.policy: ScrollBar.AlwaysOn
        TextArea {
            width: documentScroll.availableWidth
            text: content.currentDocument ? content.currentDocument.text : ""
            textFormat: TextEdit.PlainText
            font: content.compact ? Qt.font({family: Qt.application.font.family, pixelSize: Theme.fontMedium}) : Qt.application.font
            leftPadding: content.compact ? 0 : Theme.spacingMedium
            rightPadding: content.compact ? 0 : Theme.spacingMedium
            readOnly: true
            selectByMouse: true
            wrapMode: TextEdit.Wrap
        }
    }
}
