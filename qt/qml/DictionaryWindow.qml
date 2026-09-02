pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ApplicationWindow {
    id: root
    required property var hostWindow
    required property var hostPalette
    required property var backend
    required property var translator

    title: root.translator.tr("dictionary.title")
    visible: false
    width: 760
    height: 560
    minimumWidth: 760
    maximumWidth: 760
    minimumHeight: 560
    maximumHeight: 560
    transientParent: hostWindow
    modality: Qt.ApplicationModal
    flags: Qt.Dialog
    palette: hostPalette
    color: palette.window

    ListModel {
        id: dictionaryEntriesModel
    }

    function loadCurrent() {
        dictionaryEntriesModel.clear();
        const entries = root.backend.dictionaryEntries;
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
        root.backend.setDictionaryEntries(entries);
        root.hostWindow.reanalyzeAll();
        if (closeAfter) {
            root.close();
            root.visible = false;
        }
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 12
        spacing: 10

        Label {
            Layout.fillWidth: true
            text: root.translator.tr("dictionary.description")
            wrapMode: Text.WordWrap
        }

        RowLayout {
            Layout.fillWidth: true
            spacing: 8

            Label {
                Layout.preferredWidth: 280
                text: root.translator.tr("dictionary.surface")
                font.bold: true
            }
            Label {
                Layout.fillWidth: true
                text: root.translator.tr("dictionary.reading")
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
                    placeholderText: root.translator.tr("dictionary.surfaceExample")
                    text: dictionaryEntryRow.surface
                    selectByMouse: true
                    onTextEdited: dictionaryEntriesModel.setProperty(dictionaryEntryRow.index, "surface", text)
                }

                TextField {
                    Layout.fillWidth: true
                    placeholderText: root.translator.tr("dictionary.readingExample")
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
                    contentItem: BreezeIcon {
                        anchors.centerIn: parent
                        width: 18
                        height: 18
                        source: "qrc:/icons/breeze-edit-delete.svg"
                        iconColor: dictionaryDeleteButton.palette.buttonText
                    }
                    onClicked: dictionaryEntriesModel.remove(dictionaryEntryRow.index)
                    ToolTip.visible: hovered
                    ToolTip.text: root.translator.tr("dictionary.delete")
                }
            }
        }

        RowLayout {
            Layout.fillWidth: true

            Button {
                text: root.translator.tr("dictionary.addEntry")
                onClicked: root.addEntry()
            }

            Item {
                Layout.fillWidth: true
            }

            Button {
                text: root.translator.tr("common.ok")
                highlighted: true
                onClicked: root.saveCurrent(true)
            }

            Button {
                text: root.translator.tr("common.cancel")
                onClicked: {
                    root.loadCurrent();
                    root.close();
                    root.visible = false;
                }
            }

            Button {
                text: root.translator.tr("common.apply")
                onClicked: root.saveCurrent(false)
            }
        }
    }
}
