pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Dialogs
import QtQuick.Layouts
import UtauTTS.Platform 1.0

ColumnLayout {
    id: content
    required property var backend
    required property var translator
    required property var hostWindow
    readonly property bool compact: !!hostWindow && hostWindow.mobileLayout === true
    signal closeRequested()

    anchors.margins: Theme.spacingLarge
    spacing: Theme.spacingLarge


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
        dictionaryFilterField.text = "";
        dictionaryEntriesModel.append({surface: "", reading: ""});
        dictionaryTable.editCell(dictionaryEntriesModel.count - 1, 0);
    }

    function deleteSelected() {
        const row = dictionaryTable.selectedRow;
        if (row < 0 || row >= dictionaryEntriesModel.count)
            return;
        dictionaryEntriesModel.remove(row);
        dictionaryTable.selectedRow = Math.min(row, dictionaryEntriesModel.count - 1);
    }

    function currentEntries() {
        const entries = [];
        for (let index = 0; index < dictionaryEntriesModel.count; ++index) {
            const entry = dictionaryEntriesModel.get(index);
            entries.push({surface: String(entry.surface || ""), reading: String(entry.reading || "")});
        }
        return entries;
    }

    function importEntries(source) {
        const result = content.backend.importDictionary(source);
        if (!result || result._error !== undefined) {
            dictionaryMessage.text = content.translator.tr("dictionary.importFailed");
            dictionaryMessage.open();
            return;
        }
        const existing = {};
        for (const entry of content.currentEntries())
            existing[JSON.stringify([entry.surface, entry.reading])] = true;
        for (const entry of result.entries) {
            const key = JSON.stringify([entry.surface, entry.reading]);
            if (existing[key])
                continue;
            existing[key] = true;
            dictionaryEntriesModel.append({surface: entry.surface, reading: entry.reading});
        }
    }

    function exportEntries(destination) {
        if (!content.backend.exportDictionary(destination, content.currentEntries())) {
            dictionaryMessage.text = content.translator.tr("dictionary.exportFailed");
            dictionaryMessage.open();
        }
    }

    FileDialog {
        id: dictionaryImportDialog
        fileMode: FileDialog.OpenFile
        nameFilters: [content.translator.tr("dictionary.fileFilter")]
        onAccepted: content.importEntries(selectedFile)
    }

    FileDialog {
        id: dictionaryExportDialog
        fileMode: FileDialog.SaveFile
        defaultSuffix: "csv"
        nameFilters: [content.translator.tr("dictionary.fileFilter")]
        currentFile: content.backend.defaultSaveFile("dictionary.csv")
        onAccepted: content.exportEntries(selectedFile)
    }

    MessageDialog {
        id: dictionaryMessage
        buttons: MessageDialog.Ok
    }

    RowLayout {
        Layout.fillWidth: true
        spacing: Theme.spacingMedium

        TextField {
            id: dictionaryFilterField
            Layout.fillWidth: true
            Layout.preferredHeight: content.compact ? Theme.touchTarget : implicitHeight
            placeholderText: content.translator.tr("dictionary.filter")
            selectByMouse: true
        }
        Button {
            visible: Platform.hasNativeFileDialog
            text: content.translator.tr("dictionary.import")
            onClicked: dictionaryImportDialog.open()
        }
        Button {
            visible: Platform.hasNativeFileDialog
            text: content.translator.tr("dictionary.export")
            onClicked: dictionaryExportDialog.open()
        }
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

    DataTable {
        id: dictionaryTable
        objectName: "dictionaryTable"
        Layout.fillWidth: true
        Layout.fillHeight: true
        compact: content.compact
        editable: true
        columns: [
            {title: content.translator.tr("dictionary.surface"), width: Theme.fieldWidthLarge},
            {title: content.translator.tr("dictionary.reading"), width: Theme.fieldWidthLarge}
        ]
        placeholders: [content.translator.tr("dictionary.surfaceExample"),
                       content.translator.tr("dictionary.readingExample")]
        model: dictionaryEntriesModel
        cellValue: (row, column) => column === 0 ? row.surface : row.reading
        filter: {
            const query = dictionaryFilterField.text.trim().toLowerCase();
            if (!query.length)
                return null;
            return row => String(row.surface).toLowerCase().includes(query)
                    || String(row.reading).toLowerCase().includes(query);
        }
        onCellEdited: (row, column, text) => {
            if (row >= 0 && row < dictionaryEntriesModel.count)
                dictionaryEntriesModel.setProperty(row, column === 0 ? "surface" : "reading", text);
        }
        onDeleteRequested: content.deleteSelected()
    }

    GridLayout {
        Layout.fillWidth: true
        columns: content.compact ? 2 : 6
        columnSpacing: content.compact ? Theme.spacing : Theme.spacingMedium
        rowSpacing: Theme.spacing

        Button {
            Layout.fillWidth: content.compact
            Layout.preferredHeight: content.compact ? Theme.touchTarget : implicitHeight
            text: content.translator.tr("dictionary.addEntry")
            onClicked: content.addEntry()
        }

        Button {
            Layout.fillWidth: content.compact
            Layout.preferredHeight: content.compact ? Theme.touchTarget : implicitHeight
            text: content.translator.tr("dictionary.delete")
            enabled: dictionaryTable.selectedRow >= 0
            onClicked: content.deleteSelected()
        }

        Item {
            visible: !content.compact
            Layout.fillWidth: true
        }

        Button {
            Layout.fillWidth: content.compact
            Layout.preferredHeight: content.compact ? Theme.touchTarget : implicitHeight
            text: content.translator.tr("common.ok")
            highlighted: true
            onClicked: content.saveCurrent(true)
        }

        Button {
            Layout.fillWidth: content.compact
            Layout.preferredHeight: content.compact ? Theme.touchTarget : implicitHeight
            text: content.translator.tr("common.cancel")
            onClicked: {
                content.loadCurrent();
                content.closeRequested();
            }
        }

        Button {
            Layout.fillWidth: content.compact
            Layout.preferredHeight: content.compact ? Theme.touchTarget : implicitHeight
            text: content.translator.tr("common.apply")
            onClicked: content.saveCurrent(false)
        }
    }
}
