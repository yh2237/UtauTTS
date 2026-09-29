pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// ユーザー辞書編集の共有ビュー。ウィンドウ装飾は shell 側が担当する。
ColumnLayout {
    id: content
    required property var backend
    required property var translator
    required property var hostWindow
    signal closeRequested()

    anchors.margins: 12
    spacing: 10

    FontLoader {
        id: iconFont
        source: "qrc:/fonts/MaterialSymbolsOutlined-subset.ttf"
    }

    ListModel {
        id: dictionaryEntriesModel
    }

    function loadCurrent() {
        dictionaryEntriesModel.clear();
        const entries = content.backend.dictionaryEntries;
        for (let index = 0; index < entries.length; ++index) {
            const entry = entries[index] || {};
            dictionaryEntriesModel.append({
                surface: String(entry.surface || ""),
                reading: String(entry.reading || "")
            });
        }
    }

    function addEntry() {
        dictionaryEntriesModel.append({surface: "", reading: ""});
        dictionaryList.positionViewAtEnd();
    }

    function saveCurrent(closeAfter) {
        const entries = [];
        for (let index = 0; index < dictionaryEntriesModel.count; ++index) {
            const entry = dictionaryEntriesModel.get(index);
            entries.push({
                surface: String(entry.surface || "").trim(),
                reading: String(entry.reading || "").trim()
            });
        }
        content.backend.setDictionaryEntries(entries);
        if (content.hostWindow)
            content.hostWindow.reanalyzeAll();
        if (closeAfter)
            content.closeRequested();
    }

    RowLayout {
        Layout.fillWidth: true
        spacing: 8

        Label {
            Layout.preferredWidth: 280
            text: content.translator.tr("dictionary.surface")
            font.bold: true
        }
        Label {
            Layout.fillWidth: true
            text: content.translator.tr("dictionary.reading")
            font.bold: true
        }
        Item {
            Layout.preferredWidth: 32
        }
    }

    ListView {
        id: dictionaryList
        Layout.fillWidth: true
        Layout.fillHeight: true
        clip: true
        spacing: 2
        model: dictionaryEntriesModel
        ScrollBar.vertical: ScrollBar {
            id: dictionaryScrollBar
            policy: ScrollBar.AlwaysOn
        }

        delegate: RowLayout {
            id: dictionaryEntryRow
            width: Math.max(0, dictionaryList.width - 14 - 2)
            height: 36
            spacing: 4

            required property int index
            required property string surface
            required property string reading

            TextField {
                Layout.preferredWidth: 280
                placeholderText: content.translator.tr("dictionary.surfaceExample")
                text: dictionaryEntryRow.surface
                selectByMouse: true
                onTextEdited: dictionaryEntriesModel.setProperty(dictionaryEntryRow.index, "surface", text)
            }

            TextField {
                Layout.fillWidth: true
                placeholderText: content.translator.tr("dictionary.readingExample")
                text: dictionaryEntryRow.reading
                selectByMouse: true
                onTextEdited: dictionaryEntriesModel.setProperty(dictionaryEntryRow.index, "reading", text)
            }

            ToolButton {
                id: dictionaryDeleteButton
                Layout.preferredWidth: 24
                Layout.minimumWidth: 24
                Layout.maximumWidth: 24
                Layout.preferredHeight: 24
                Layout.alignment: Qt.AlignVCenter
                contentItem: Text {
                    anchors.centerIn: parent
                    width: 18
                    height: 18
                    text: "\ue5cd"
                    color: dictionaryDeleteButton.palette.buttonText
                    font.family: iconFont.name
                    font.pixelSize: 20
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
                onClicked: dictionaryEntriesModel.remove(dictionaryEntryRow.index)
                ToolTip.visible: hovered
                ToolTip.text: content.translator.tr("dictionary.delete")
            }
        }
    }

    RowLayout {
        Layout.fillWidth: true

        Button {
            text: content.translator.tr("dictionary.addEntry")
            onClicked: content.addEntry()
        }

        Item {
            Layout.fillWidth: true
        }

        Button {
            text: content.translator.tr("common.ok")
            highlighted: true
            onClicked: content.saveCurrent(true)
        }

        Button {
            text: content.translator.tr("common.cancel")
            onClicked: {
                content.loadCurrent();
                content.closeRequested();
            }
        }

        Button {
            text: content.translator.tr("common.apply")
            onClicked: content.saveCurrent(false)
        }
    }
}
