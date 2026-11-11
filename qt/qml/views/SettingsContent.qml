pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Dialogs
import QtQuick.Layouts
import "../SettingsIO.js" as SettingsIO
import UtauTTS.Platform 1.0
import UtauTTSStyle

Item {
    id: root
    required property var hostWindow
    required property var hostPalette
    required property var backend
    required property var translator
    readonly property bool compact: width < 600
    property var audioOutputDevices: []
    signal applyRequested(bool closeAfter)
    signal closeRequested()

    property int currentPage: 0
    SettingsDraft { id: draftObject }
    // draftは、設定画面の編集中の値。保存時にまとめて書き出す。
    property QtObject draft: draftObject
    property alias pendingDefaultVoicebankId: draftObject.defaultVoicebankId
    property alias pendingDefaultModelId: draftObject.defaultModelId
    property alias pendingDefaultAliasPolicy: draftObject.defaultAliasPolicy
    property alias pendingDefaultTone: draftObject.defaultTone
    property alias pendingExportTextEncoding: draftObject.exportTextEncoding
    property alias pendingDarkMode: draftObject.darkMode
    property alias pendingLanguage: draftObject.language
    property alias pendingFfmpegPath: draftObject.ffmpegPath
    property alias pendingAudioOutputDeviceId: draftObject.audioOutputDeviceId
    property alias pendingCloseLogOnSuccess: draftObject.closeLogOnSuccess
    property alias pendingUpdateCheckEnabled: draftObject.updateCheckEnabled
    property alias pendingPreReleaseUpdateCheckEnabled: draftObject.preReleaseUpdateCheckEnabled
    property alias pendingPreviewCacheFileCount: draftObject.previewCacheFileCount
    property alias pendingAutoPreviewEnabled: draftObject.autoPreviewEnabled
    property alias pendingExtendedDetailsVisible: draftObject.extendedDetailsVisible
    property alias pendingSynthesizeShortcut: draftObject.synthesizeShortcut
    property alias pendingSaveProjectShortcut: draftObject.saveProjectShortcut
    property alias pendingReloadVoicebanksShortcut: draftObject.reloadVoicebanksShortcut
    property alias pendingAddUtteranceShortcut: draftObject.addUtteranceShortcut
    property alias pendingRemoveUtteranceShortcut: draftObject.removeUtteranceShortcut
    property alias pendingUndoShortcut: draftObject.undoShortcut
    property alias pendingRedoShortcut: draftObject.redoShortcut
    property var languageCodes: root.backend.languageCodes()
    function languageLabels() {
        const labels = [];
        for (let index = 0; index < root.languageCodes.length; ++index) {
            const code = root.languageCodes[index];
            labels.push(code === "auto" ? root.translator.tr("settings.language.auto")
                                        : root.backend.languageDisplayName(code));
        }
        return labels;
    }

    function settingsPageLabels() {
        return [root.translator.tr("settings.page.appearance"),
            root.translator.tr("settings.page.synthesis"),
            root.translator.tr("settings.page.shortcuts"),
            root.translator.tr("settings.page.indexes")];
    }

    // 参照照合でも使う設定。方式ごとの欄の上に並べる。
    readonly property var commonSettingIds: ["intonation_strength", "mora_duration_ms"]

    function commonSettings() {
        const rows = [];
        const settings = root.backend.settingDefinitions;
        for (let order = 0; order < root.commonSettingIds.length; ++order) {
            for (let index = 0; index < settings.length; ++index) {
                if (String(settings[index].id) === root.commonSettingIds[order])
                    rows.push(settings[index]);
            }
        }
        return rows;
    }

    Component {
        id: settingRowComponent
        RowLayout {
            id: settingRow
            required property var modelData
            Layout.fillWidth: true
            property string settingType: String(settingRow.modelData.type || "integer")
            property bool scaled: settingRow.settingType === "number"
            property real factor: settingRow.scaled ? 100 : 1

        function settingFallback() {
            return settingRow.modelData.default;
        }
        function storedValue() {
            // 変更の通知を受けるためにsynthesisSettingsを読む。
            root.backend.synthesisSettings;
            return root.backend.synthesisSetting(
                String(settingRow.modelData.id),
                settingRow.settingFallback());
        }
        function minimum() {
            return settingRow.modelData.min !== undefined
                    ? Number(settingRow.modelData.min) : 0;
        }
        function maximum() {
            if (settingRow.modelData.max !== undefined)
                return Number(settingRow.modelData.max);
            return settingRow.scaled ? 1 : 100;
        }
        function stepSize() {
            const step = settingRow.modelData.step !== undefined
                    ? Number(settingRow.modelData.step) : 1;
            const scaledStep = step * settingRow.factor;
            return scaledStep > 0 ? scaledStep : 1;
        }
        function scaledValue() {
            return Math.round(Number(settingRow.storedValue())
                              * settingRow.factor);
        }
        function actualValue(value) {
            return settingRow.scaled
                    ? value / settingRow.factor : value;
        }
        function textValue(value) {
            return settingRow.scaled
                    ? (value / settingRow.factor).toFixed(2)
                    : String(value);
        }
        function parsedValue(text) {
            const parsed = settingRow.scaled
                    ? parseFloat(text) : parseInt(text);
            return isNaN(parsed)
                    ? 0
                    : Math.round(parsed * settingRow.factor);
        }
        function applyValue(value) {
            root.backend.setSynthesisSetting(
                String(settingRow.modelData.id),
                settingRow.actualValue(value));
        }
        function enumModel() {
            const options = settingRow.modelData.options || [];
            const rows = [];
            for (let index = 0; index < options.length; ++index) {
                const option = options[index];
                rows.push({
                    value: String(option.value),
                    label: root.translator.tr(option.label || option.value)
                });
            }
            return rows;
        }

        Label {
            Layout.fillWidth: true
            text: root.translator.tr(
                      settingRow.modelData.label
                      || settingRow.modelData.id)
        }
        SpinBox {
            visible: settingRow.settingType === "integer"
                     || settingRow.settingType === "number"
            Layout.preferredWidth: Theme.fieldWidth
            Layout.alignment: Qt.AlignVCenter
            from: Math.round(settingRow.minimum()
                             * settingRow.factor)
            to: Math.round(settingRow.maximum()
                           * settingRow.factor)
            stepSize: settingRow.stepSize()
            value: settingRow.scaledValue()
            editable: true
            textFromValue: value => settingRow.textValue(value)
            valueFromText: text => settingRow.parsedValue(text)
            onValueModified: settingRow.applyValue(value)
        }
        Switch {
            visible: settingRow.settingType === "boolean"
            Layout.alignment: Qt.AlignVCenter
            checked: !!settingRow.storedValue()
            onToggled: root.backend.setSynthesisSetting(
                String(settingRow.modelData.id),
                checked)
        }
        ComboBox {
            visible: settingRow.settingType === "enum"
            Layout.preferredWidth: Theme.fieldWidthLarge
            model: settingRow.enumModel()
            textRole: "label"
            valueRole: "value"
            currentIndex: count > 0 ? indexOfValue(String(settingRow.storedValue())) : -1
            onActivated: root.backend.setSynthesisSetting(
                String(settingRow.modelData.id),
                currentValue)
        }
        SettingsResetButton {
            translator: root.translator
            onResetRequested: root.backend.setSynthesisSetting(
                String(settingRow.modelData.id),
                settingRow.modelData.default)
        }
    }
    }

    function settingGroups() {
        const groups = [];
        const settings = root.backend.settingDefinitions;
        for (let index = 0; index < settings.length; ++index) {
            const setting = settings[index];
            if (root.commonSettingIds.indexOf(String(setting.id)) >= 0)
                continue;
            const group = String(setting.group || "");
            let entry = null;
            for (let scan = 0; scan < groups.length; ++scan) {
                if (groups[scan].id === group) {
                    entry = groups[scan];
                    break;
                }
            }
            if (!entry) {
                entry = { id: group, settings: [] };
                groups.push(entry);
            }
            entry.settings.push(setting);
        }
        return groups;
    }

    function groupLabel(group) {
        const key = "settings.group." + group;
        const text = root.translator.tr(key);
        return text === key ? group : text;
    }

    FolderDialog {
        id: ffmpegFolderDialog
        title: root.translator.tr("settings.ffmpegPath.choose")
        onAccepted: {
            const path = selectedFolder.toLocalFile();
            if (path.length)
                root.pendingFfmpegPath = path;
        }
    }

    function referenceIndexVoicebankName(voicebank) {
        const voicebanks = root.backend.voicebanks;
        for (let index = 0; index < voicebanks.length; ++index) {
            if (String(voicebanks[index].id) === String(voicebank))
                return String(voicebanks[index].name);
        }
        return String(voicebank || "");
    }

    function loadCurrent() {
        root.backend.refreshReferenceIndexes();
        SettingsIO.loadCurrent(draftObject, root.backend, {
            voicebank: value => root.validDefaultVoicebankId(value),
            model: value => root.validDefaultModelId(value)
        });
    }

    function resetDefaultVoicebank() { SettingsIO.reset(draftObject, "defaultVoicebankId"); }
    function resetDefaultAliasPolicy() { SettingsIO.reset(draftObject, "defaultAliasPolicy"); }
    function resetDefaultModel() { SettingsIO.reset(draftObject, "defaultModelId", root.backend); }
    function resetDefaultTone() { SettingsIO.reset(draftObject, "defaultTone"); }
    function resetPreviewCacheFileCount() { SettingsIO.reset(draftObject, "previewCacheFileCount"); }
    function resetExportTextEncoding() { SettingsIO.reset(draftObject, "exportTextEncoding"); }
    function resetTheme() { SettingsIO.reset(draftObject, "darkMode"); }
    function resetLanguage() { SettingsIO.reset(draftObject, "language"); }
    function resetFfmpegPath() { SettingsIO.reset(draftObject, "ffmpegPath"); }
    function resetAudioOutputDevice() { SettingsIO.reset(draftObject, "audioOutputDeviceId"); }
    function resetUpdateCheckEnabled() { SettingsIO.reset(draftObject, "updateCheckEnabled"); }
    function resetPreReleaseUpdateCheckEnabled() { SettingsIO.reset(draftObject, "preReleaseUpdateCheckEnabled"); }
    function resetAutoPreviewEnabled() { SettingsIO.reset(draftObject, "autoPreviewEnabled"); }
    function resetExtendedDetailsVisible() { SettingsIO.reset(draftObject, "extendedDetailsVisible"); }
    function resetCloseLogOnSuccess() { SettingsIO.reset(draftObject, "closeLogOnSuccess"); }
    function resetSynthesizeShortcut() { SettingsIO.reset(draftObject, "synthesizeShortcut"); }
    function resetSaveProjectShortcut() { SettingsIO.reset(draftObject, "saveProjectShortcut"); }
    function resetReloadVoicebanksShortcut() { SettingsIO.reset(draftObject, "reloadVoicebanksShortcut"); }
    function resetAddUtteranceShortcut() { SettingsIO.reset(draftObject, "addUtteranceShortcut"); }
    function resetRemoveUtteranceShortcut() { SettingsIO.reset(draftObject, "removeUtteranceShortcut"); }
    function resetUndoShortcut() { SettingsIO.reset(draftObject, "undoShortcut"); }
    function resetRedoShortcut() { SettingsIO.reset(draftObject, "redoShortcut"); }

    function defaultVoicebankIndex() {
        let id = String(root.pendingDefaultVoicebankId || "");
        // 選んでいなければ、同梱の足立レイ（AppCore.standardVoicebank）を示す。
        for (let index = 0; !id.length && index < root.backend.voicebanks.length; ++index) {
            const voice = root.backend.voicebanks[index];
            if (String(voice.id || "").indexOf("足立レイ") === 0 || String(voice.name || "").indexOf("足立レイ") >= 0)
                id = String(voice.id);
        }
        for (let index = 0; index < root.backend.voicebanks.length; ++index) {
            if (root.backend.voicebanks[index].id === id)
                return index;
        }
        return root.backend.voicebanks.length ? 0 : -1;
    }

    function validDefaultVoicebankId(value) {
        const id = String(value || "");
        for (let index = 0; index < root.backend.voicebanks.length; ++index)
            if (root.backend.voicebanks[index].id === id)
                return id;
        return "";
    }

    function validDefaultModelId(value) {
        const id = String(value || "none");
        if (id === "none")
            return id;
        for (let index = 0; index < root.backend.models.length; ++index)
            if (root.backend.models[index].id === id)
                return id;
        return root.backend.models.length ? root.backend.models[0].id : "none";
    }

    function defaultModelIndex() {
        if (root.pendingDefaultModelId === "none")
            return 0;
        for (let index = 0; index < root.backend.models.length; ++index)
            if (root.backend.models[index].id === root.pendingDefaultModelId)
                return index + 1;
        return root.backend.models.length ? 1 : 0;
    }

    function audioOutputDeviceKey(device) {
        if (!device)
            return "";
        let key = "";
        if (device.id !== undefined && device.id !== null)
            key = String(root.backend.audioOutputDeviceKey(device.id) || "");
        if (!key.length && device.description)
            key = "description:" + String(device.description);
        return key;
    }

    function audioOutputDeviceModel() {
        const rows = [{
            id: "",
            name: root.translator.tr("settings.audioOutput.default")
        }];
        const devices = root.audioOutputDevices || [];
        for (let index = 0; index < devices.length; ++index) {
            const device = devices[index];
            const id = root.audioOutputDeviceKey(device);
            if (!id.length)
                continue;
            rows.push({
                id: id,
                name: String(device.description || id)
            });
        }
        return rows;
    }

    function audioOutputDeviceIndex() {
        const id = String(root.pendingAudioOutputDeviceId || "");
        if (!id.length)
            return 0;
        const rows = root.audioOutputDeviceModel();
        for (let index = 1; index < rows.length; ++index) {
            if (rows[index].id === id)
                return index;
        }
        return 0;
    }

    function shortcutFromEvent(event) {
        const key = event.key;
        if (key === Qt.Key_Control || key === Qt.Key_Shift || key === Qt.Key_Alt || key === Qt.Key_Meta)
            return "";

        const parts = [];
        if (event.modifiers & Qt.ControlModifier)
            parts.push("Ctrl");
        if (event.modifiers & Qt.AltModifier)
            parts.push("Alt");
        if (event.modifiers & Qt.ShiftModifier)
            parts.push("Shift");
        if (event.modifiers & Qt.MetaModifier)
            parts.push("Meta");

        let keyName = "";
        if (key >= Qt.Key_A && key <= Qt.Key_Z)
            keyName = String.fromCharCode(key);
        else if (key >= Qt.Key_0 && key <= Qt.Key_9)
            keyName = String.fromCharCode(key);
        else if (key >= Qt.Key_F1 && key <= Qt.Key_F35)
            keyName = "F" + (key - Qt.Key_F1 + 1);
        else {
            switch (key) {
            case Qt.Key_Return:
            case Qt.Key_Enter: keyName = "Enter"; break;
            case Qt.Key_Space: keyName = "Space"; break;
            case Qt.Key_Tab:
            case Qt.Key_Backtab: keyName = "Tab"; break;
            case Qt.Key_Escape: keyName = "Esc"; break;
            case Qt.Key_Left: keyName = "Left"; break;
            case Qt.Key_Right: keyName = "Right"; break;
            case Qt.Key_Up: keyName = "Up"; break;
            case Qt.Key_Down: keyName = "Down"; break;
            case Qt.Key_Home: keyName = "Home"; break;
            case Qt.Key_End: keyName = "End"; break;
            case Qt.Key_PageUp: keyName = "PageUp"; break;
            case Qt.Key_PageDown: keyName = "PageDown"; break;
            case Qt.Key_Insert: keyName = "Insert"; break;
            case Qt.Key_Delete: keyName = "Delete"; break;
            case Qt.Key_Plus: keyName = "Plus"; break;
            case Qt.Key_Minus: keyName = "Minus"; break;
            case Qt.Key_Comma: keyName = "Comma"; break;
            case Qt.Key_Period: keyName = "Period"; break;
            }
        }
        return keyName ? parts.concat([keyName]).join("+") : "";
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: Theme.spacingLarge
        spacing: 0

        TabBar {
            id: settingsTabs
            z: 1
            currentIndex: root.currentPage
            onCurrentIndexChanged: root.currentPage = currentIndex

            Repeater {
                model: root.settingsPageLabels()
                delegate: TabButton {
                    required property string modelData
                    width: implicitWidth
                    text: modelData
                }
            }
        }

        TabFrame {
            id: settingsFrame
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.topMargin: -overlap
            contentMargin: frameWidth

            StackLayout {
                anchors.fill: parent
                currentIndex: root.currentPage

                ScrollView {
                    id: appearanceSettingsPage
                    contentWidth: availableWidth
                    contentHeight: appearanceSettingsColumn.implicitHeight + 2 * Theme.spacingLarge
                    rightPadding: ScrollBar.vertical.width

                    ColumnLayout {
                        id: appearanceSettingsColumn
                        x: Theme.spacingLarge
                        y: Theme.spacingLarge
                        width: appearanceSettingsPage.availableWidth - 2 * Theme.spacingLarge
                        spacing: Theme.spacing

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.theme")
                            }
                            ComboBox {
                                id: themeCombo
                                Layout.preferredWidth: Theme.fieldWidth
                                model: [root.translator.tr("settings.theme.light"), root.translator.tr("settings.theme.dark")]
                                currentIndex: root.pendingDarkMode ? 1 : 0
                                onActivated: root.pendingDarkMode = currentIndex === 1
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetTheme()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.language")
                            }
                            ComboBox {
                                id: languageCombo
                                Layout.preferredWidth: Theme.fieldWidth
                                model: root.languageLabels()
                                currentIndex: root.languageCodes.indexOf(root.pendingLanguage)
                                onActivated: root.pendingLanguage = root.languageCodes[currentIndex]
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetLanguage()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.audioOutput")
                            }
                            ComboBox {
                                id: audioOutputCombo
                                Layout.preferredWidth: Theme.fieldWidthLarge
                                model: root.audioOutputDeviceModel()
                                textRole: "name"
                                valueRole: "id"
                                currentIndex: root.audioOutputDeviceIndex()
                                onActivated: root.pendingAudioOutputDeviceId = currentValue
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetAudioOutputDevice()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            visible: Platform.hasExternalTools
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.ffmpegPath")
                            }
                            TextField {
                                id: ffmpegPathField
                                Layout.fillWidth: true
                                placeholderText: root.translator.tr("settings.ffmpegPath.placeholder")
                                text: root.pendingFfmpegPath
                                selectByMouse: true
                                onTextEdited: root.pendingFfmpegPath = text
                            }
                            Button {
                                text: root.translator.tr("settings.ffmpegPath.choose")
                                onClicked: ffmpegFolderDialog.open()
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetFfmpegPath()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.updateCheckEnabled")
                            }
                            Switch {
                                id: updateCheckSwitch
                                Layout.alignment: Qt.AlignVCenter | Qt.AlignRight
                                checked: root.pendingUpdateCheckEnabled
                                onToggled: root.pendingUpdateCheckEnabled = checked
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetUpdateCheckEnabled()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.preReleaseUpdateCheckEnabled")
                            }
                            Switch {
                                id: preReleaseUpdateSwitch
                                Layout.alignment: Qt.AlignVCenter | Qt.AlignRight
                                checked: root.pendingPreReleaseUpdateCheckEnabled
                                onToggled: root.pendingPreReleaseUpdateCheckEnabled = checked
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetPreReleaseUpdateCheckEnabled()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.autoPreview")
                            }
                            Switch {
                                id: autoPreviewSwitch
                                Layout.alignment: Qt.AlignVCenter | Qt.AlignRight
                                checked: root.pendingAutoPreviewEnabled
                                onToggled: root.pendingAutoPreviewEnabled = checked
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetAutoPreviewEnabled()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.extendedDetails")
                            }
                            Switch {
                                id: extendedDetailsSwitch
                                Layout.alignment: Qt.AlignVCenter | Qt.AlignRight
                                checked: root.pendingExtendedDetailsVisible
                                onToggled: root.pendingExtendedDetailsVisible = checked
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetExtendedDetailsVisible()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.closeLogOnSuccess")
                            }
                            Switch {
                                id: closeLogOnSuccessSwitch
                                Layout.alignment: Qt.AlignVCenter | Qt.AlignRight
                                checked: root.pendingCloseLogOnSuccess
                                onToggled: root.pendingCloseLogOnSuccess = checked
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetCloseLogOnSuccess()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                text: root.translator.tr("settings.previewCacheFileCount")
                                Layout.fillWidth: true
                            }
                            SpinBox {
                                id: previewCacheSpin
                                Layout.preferredWidth: Theme.fieldWidth
                                from: 1
                                to: 256
                                value: root.pendingPreviewCacheFileCount
                                editable: true
                                property string unitText: root.translator.tr("settings.previewCacheFileCount.unit")
                                textFromValue: value => value + " " + previewCacheSpin.unitText
                                Binding {
                                    target: previewCacheSpin.contentItem
                                    property: "text"
                                    value: previewCacheSpin.textFromValue(
                                               previewCacheSpin.value,
                                               previewCacheSpin.locale)
                                }
                                valueFromText: text => parseInt(text)
                                onValueModified: root.pendingPreviewCacheFileCount = value
                                TapHandler {
                                    acceptedButtons: Qt.LeftButton
                                    grabPermissions: PointerHandler.CanTakeOverFromAnything
                                    onDoubleTapped: root.pendingPreviewCacheFileCount = 32
                                }
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetPreviewCacheFileCount()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.exportTextEncoding")
                            }
                            ComboBox {
                                id: exportTextEncodingCombo
                                Layout.preferredWidth: Theme.fieldWidth
                                model: ["UTF-8", "Shift_JIS (CP932)"]
                                currentIndex: root.pendingExportTextEncoding === "shift_jis" ? 1 : 0
                                onActivated: root.pendingExportTextEncoding = currentIndex === 1 ? "shift_jis" : "utf-8"
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetExportTextEncoding()
                            }
                        }

                    }
                }

                Item {
                    id: timingSettingsPage

                    ColumnLayout {
                        anchors.fill: parent
                        spacing: Theme.spacing

                        ScrollView {
                            id: synthesisSettingsPage
                            Layout.fillWidth: true
                            Layout.fillHeight: true
                            contentWidth: availableWidth
                            contentHeight: synthesisSettingsColumn.implicitHeight + 2 * Theme.spacingLarge
                            rightPadding: ScrollBar.vertical.width

                            ColumnLayout {
                                id: synthesisSettingsColumn
                                x: Theme.spacingLarge
                                y: Theme.spacingLarge
                                width: synthesisSettingsPage.availableWidth - 2 * Theme.spacingLarge
                                spacing: Theme.spacingXLarge

                                ColumnLayout {
                                    Layout.fillWidth: true
                                    spacing: Theme.spacing

                                    Label {
                                        Layout.fillWidth: true
                                        text: root.translator.tr("settings.synthesisGeneral")
                                        font.bold: true
                                    }

                                        RowLayout {
                                            Layout.fillWidth: true
                                            Label {
                                                text: root.translator.tr("settings.defaultVoicebank")
                                                Layout.fillWidth: true
                                            }
                                            ComboBox {
                                                id: defaultVoicebankCombo
                                                Layout.preferredWidth: Theme.fieldWidthLarge
                                                model: root.backend.voicebanks
                                                textRole: "name"
                                                valueRole: "id"
                                                currentIndex: root.defaultVoicebankIndex()
                                                onActivated: root.pendingDefaultVoicebankId = currentValue
                                            }
                                            SettingsResetButton {
                                                translator: root.translator
                                                onResetRequested: root.resetDefaultVoicebank()
                                            }
                                        }
                                        RowLayout {
                                            Layout.fillWidth: true
                                            Label {
                                                text: root.translator.tr("settings.defaultTone")
                                                Layout.fillWidth: true
                                            }
                                            ToneSpinBox {
                                                id: defaultToneInput
                                                Layout.preferredWidth: Theme.fieldWidth
                                                automaticText: root.translator.tr("main.aliasPolicy.auto")
                                                onToneModified: tone => root.pendingDefaultTone = tone
                                                Binding on value {
                                                    value: defaultToneInput.toneToValue(root.pendingDefaultTone)
                                                }
                                            }
                                            SettingsResetButton {
                                                translator: root.translator
                                                onResetRequested: root.resetDefaultTone()
                                            }
                                        }
                                        Repeater {
                                            model: root.commonSettings()
                                            delegate: settingRowComponent
                                        }
                                    }
                                    ColumnLayout {
                                        Layout.fillWidth: true
                                        spacing: Theme.spacing

                                        Label {
                                            Layout.fillWidth: true
                                            text: root.translator.tr("main.method.native")
                                            font.bold: true
                                        }
                                        RowLayout {
                                            Layout.fillWidth: true
                                            Label {
                                                text: root.translator.tr("settings.defaultAliasPolicy")
                                                Layout.fillWidth: true
                                            }
                                            ComboBox {
                                                id: defaultAliasPolicyCombo
                                                Layout.preferredWidth: Theme.fieldWidthLarge
                                                model: [
                                                    { id: "auto", display_name: root.translator.tr("main.aliasPolicy.auto") },
                                                    { id: "cvvc-enhanced", display_name: root.translator.tr("main.aliasPolicy.cvvcEnhanced") },
                                                    { id: "vcv-prefer", display_name: root.translator.tr("main.aliasPolicy.vcvPrefer") },
                                                    { id: "cvvc-prefer", display_name: root.translator.tr("main.aliasPolicy.cvvcPrefer") },
                                                    { id: "cv-only", display_name: root.translator.tr("main.aliasPolicy.cvOnly") }
                                                ]
                                                textRole: "display_name"
                                                valueRole: "id"
                                                currentIndex: count > 0 ? indexOfValue(root.pendingDefaultAliasPolicy) : -1
                                                onActivated: root.pendingDefaultAliasPolicy = currentValue
                                            }
                                            SettingsResetButton {
                                                translator: root.translator
                                                onResetRequested: root.resetDefaultAliasPolicy()
                                            }
                                        }
                                        RowLayout {
                                            Layout.fillWidth: true
                                            Label {
                                                text: root.translator.tr("settings.defaultModel")
                                                Layout.fillWidth: true
                                            }
                                            ComboBox {
                                                id: defaultModelCombo
                                                Layout.preferredWidth: Theme.fieldWidthLarge
                                                model: [{
                                                    id: "none",
                                                    display_name: root.translator.tr("main.modelNone")
                                                }].concat(root.backend.models)
                                                textRole: "display_name"
                                                valueRole: "id"
                                                currentIndex: root.defaultModelIndex()
                                                onActivated: root.pendingDefaultModelId = currentValue
                                            }
                                            SettingsResetButton {
                                                translator: root.translator
                                                onResetRequested: root.resetDefaultModel()
                                            }
                                        }
                                    }
                                    Repeater {
                                        model: root.settingGroups()
                                        delegate: ColumnLayout {
                                            id: settingGroup
                                            required property var modelData
                                            Layout.fillWidth: true
                                            spacing: Theme.spacing

                                            Label {
                                                Layout.fillWidth: true
                                                text: root.groupLabel(String(settingGroup.modelData.id))
                                                font.bold: true
                                            }

                                            Repeater {
                                                model: settingGroup.modelData.settings
                                                delegate: settingRowComponent
                                        }
                                    }
                                }
                            }
                        }
                    }
                }

                ScrollView {
                    id: shortcutSettingsPage
                    contentWidth: availableWidth
                    contentHeight: shortcutSettingsColumn.implicitHeight + 2 * Theme.spacingLarge
                    rightPadding: ScrollBar.vertical.width

                    ColumnLayout {
                        id: shortcutSettingsColumn
                        x: Theme.spacingLarge
                        y: Theme.spacingLarge
                        width: shortcutSettingsPage.availableWidth - 2 * Theme.spacingLarge
                        spacing: Theme.spacingLarge

                        Label {
                            Layout.fillWidth: true
                            text: root.translator.tr("settings.shortcutHint")
                            wrapMode: Text.WordWrap
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.shortcut.undo")
                            }
                            TextField {
                                id: undoShortcutField
                                Layout.preferredWidth: Theme.fieldWidth
                                text: root.pendingUndoShortcut
                                readOnly: true
                                selectByMouse: false
                                onActiveFocusChanged: if (activeFocus) selectAll()
                                Keys.onPressed: event => {
                                    if (event.key === Qt.Key_Backspace || event.key === Qt.Key_Delete) {
                                        root.pendingUndoShortcut = "";
                                        event.accepted = true;
                                        return;
                                    }
                                    const sequence = root.shortcutFromEvent(event);
                                    if (sequence.length) {
                                        root.pendingUndoShortcut = sequence;
                                        event.accepted = true;
                                    }
                                }
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetUndoShortcut()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.shortcut.redo")
                            }
                            TextField {
                                id: redoShortcutField
                                Layout.preferredWidth: Theme.fieldWidth
                                text: root.pendingRedoShortcut
                                readOnly: true
                                selectByMouse: false
                                onActiveFocusChanged: if (activeFocus) selectAll()
                                Keys.onPressed: event => {
                                    if (event.key === Qt.Key_Backspace || event.key === Qt.Key_Delete) {
                                        root.pendingRedoShortcut = "";
                                        event.accepted = true;
                                        return;
                                    }
                                    const sequence = root.shortcutFromEvent(event);
                                    if (sequence.length) {
                                        root.pendingRedoShortcut = sequence;
                                        event.accepted = true;
                                    }
                                }
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetRedoShortcut()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.shortcut.synthesize")
                            }
                            TextField {
                                id: synthesizeShortcutField
                                Layout.preferredWidth: Theme.fieldWidth
                                text: root.pendingSynthesizeShortcut
                                readOnly: true
                                selectByMouse: false
                                onActiveFocusChanged: if (activeFocus) selectAll()
                                Keys.onPressed: event => {
                                    if (event.key === Qt.Key_Backspace || event.key === Qt.Key_Delete) {
                                        root.pendingSynthesizeShortcut = "";
                                        event.accepted = true;
                                        return;
                                    }
                                    const sequence = root.shortcutFromEvent(event);
                                    if (sequence.length) {
                                        root.pendingSynthesizeShortcut = sequence;
                                        event.accepted = true;
                                    }
                                }
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetSynthesizeShortcut()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.shortcut.saveProject")
                            }
                            TextField {
                                id: saveProjectShortcutField
                                Layout.preferredWidth: Theme.fieldWidth
                                text: root.pendingSaveProjectShortcut
                                readOnly: true
                                selectByMouse: false
                                onActiveFocusChanged: if (activeFocus) selectAll()
                                Keys.onPressed: event => {
                                    if (event.key === Qt.Key_Backspace || event.key === Qt.Key_Delete) {
                                        root.pendingSaveProjectShortcut = "";
                                        event.accepted = true;
                                        return;
                                    }
                                    const sequence = root.shortcutFromEvent(event);
                                    if (sequence.length) {
                                        root.pendingSaveProjectShortcut = sequence;
                                        event.accepted = true;
                                    }
                                }
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetSaveProjectShortcut()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.shortcut.reloadVoicebanks")
                            }
                            TextField {
                                id: reloadVoicebanksShortcutField
                                Layout.preferredWidth: Theme.fieldWidth
                                text: root.pendingReloadVoicebanksShortcut
                                readOnly: true
                                selectByMouse: false
                                onActiveFocusChanged: if (activeFocus) selectAll()
                                Keys.onPressed: event => {
                                    if (event.key === Qt.Key_Backspace || event.key === Qt.Key_Delete) {
                                        root.pendingReloadVoicebanksShortcut = "";
                                        event.accepted = true;
                                        return;
                                    }
                                    const sequence = root.shortcutFromEvent(event);
                                    if (sequence.length) {
                                        root.pendingReloadVoicebanksShortcut = sequence;
                                        event.accepted = true;
                                    }
                                }
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetReloadVoicebanksShortcut()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.shortcut.addUtterance")
                            }
                            TextField {
                                id: addUtteranceShortcutField
                                Layout.preferredWidth: Theme.fieldWidth
                                text: root.pendingAddUtteranceShortcut
                                readOnly: true
                                selectByMouse: false
                                onActiveFocusChanged: if (activeFocus) selectAll()
                                Keys.onPressed: event => {
                                    if (event.key === Qt.Key_Backspace || event.key === Qt.Key_Delete) {
                                        root.pendingAddUtteranceShortcut = "";
                                        event.accepted = true;
                                        return;
                                    }
                                    const sequence = root.shortcutFromEvent(event);
                                    if (sequence.length) {
                                        root.pendingAddUtteranceShortcut = sequence;
                                        event.accepted = true;
                                    }
                                }
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetAddUtteranceShortcut()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.shortcut.removeUtterance")
                            }
                            TextField {
                                id: removeUtteranceShortcutField
                                Layout.preferredWidth: Theme.fieldWidth
                                text: root.pendingRemoveUtteranceShortcut
                                readOnly: true
                                selectByMouse: false
                                onActiveFocusChanged: if (activeFocus) selectAll()
                                Keys.onPressed: event => {
                                    if (event.key === Qt.Key_Backspace) {
                                        root.pendingRemoveUtteranceShortcut = "";
                                        event.accepted = true;
                                        return;
                                    }
                                    const sequence = root.shortcutFromEvent(event);
                                    if (sequence.length) {
                                        root.pendingRemoveUtteranceShortcut = sequence;
                                        event.accepted = true;
                                    }
                                }
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetRemoveUtteranceShortcut()
                            }
                        }
                    }
                }

                Item {
                    id: referenceIndexesSettingsPage
                    readonly property var selectedIndex: referenceIndexTable.selectedRow >= 0
                            ? root.backend.referenceIndexes[referenceIndexTable.selectedRow] : undefined
                    readonly property bool editable: !root.backend.busy && !root.backend.referenceIndexBuilding

                    function deleteSelected() {
                        if (selectedIndex && editable)
                            root.backend.deleteReferenceIndex(String(selectedIndex.name));
                    }

                    ColumnLayout {
                        anchors.fill: parent
                        anchors.margins: Theme.spacingLarge
                        spacing: Theme.spacing

                        RowLayout {
                            Layout.fillWidth: true
                            spacing: Theme.spacingMedium
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.referenceIndexes.title")
                                font.bold: true
                                wrapMode: Text.WordWrap
                            }
                            ComboBox {
                                id: referenceIndexVoicebankCombo
                                Layout.preferredWidth: Theme.fieldWidthLarge
                                model: root.backend.voicebanks
                                textRole: "name"
                                valueRole: "id"
                                currentIndex: root.defaultVoicebankIndex()
                            }
                            Button {
                                text: root.translator.tr("settings.referenceIndexes.create")
                                enabled: !!referenceIndexVoicebankCombo.currentValue && referenceIndexesSettingsPage.editable
                                onClicked: root.backend.buildReferenceIndex(String(referenceIndexVoicebankCombo.currentValue), "")
                            }
                            Button {
                                text: root.translator.tr("settings.referenceIndexes.delete")
                                enabled: !!referenceIndexesSettingsPage.selectedIndex && referenceIndexesSettingsPage.editable
                                onClicked: referenceIndexesSettingsPage.deleteSelected()
                            }
                        }
                        ProgressBar {
                            Layout.fillWidth: true
                            visible: root.backend.referenceIndexBuilding
                            from: 0
                            to: 1
                            value: root.backend.referenceIndexProgress
                        }
                        DataTable {
                            id: referenceIndexTable
                            objectName: "referenceIndexTable"
                            Layout.fillWidth: true
                            Layout.fillHeight: true
                            compact: root.compact
                            columns: [
                                {title: root.translator.tr("settings.referenceIndexes.voicebank"), width: 160},
                                {title: root.translator.tr("settings.referenceIndexes.model"), width: 140},
                                {title: root.translator.tr("settings.referenceIndexes.version"), width: 48},
                                {title: root.translator.tr("settings.referenceIndexes.size"), width: 80},
                                {title: root.translator.tr("settings.referenceIndexes.modified"), width: 160}
                            ]
                            model: root.backend.referenceIndexes
                            cellValue: (row, column) => {
                                const item = row || {};
                                switch (column) {
                                case 0: return root.referenceIndexVoicebankName(item.voicebank);
                                case 1: return String(item.model || "");
                                case 2: return String(item.version ?? "");
                                case 3: return root.translator.tr("settings.referenceIndexes.sizeMb",
                                                                  (Number(item.size_bytes || 0) / (1024 * 1024)).toFixed(1));
                                default: return Qt.formatDateTime(new Date(Number(item.modified_ms)), Qt.DefaultLocaleShortDate);
                                }
                            }
                            sortValue: (row, column) => {
                                switch (column) {
                                case 2: return Number(row.version ?? 0);
                                case 3: return Number(row.size_bytes || 0);
                                case 4: return Number(row.modified_ms || 0);
                                default: return referenceIndexTable.cellValue(row, column);
                                }
                            }
                            emptyText: root.translator.tr("settings.referenceIndexes.empty")
                            onDeleteRequested: referenceIndexesSettingsPage.deleteSelected()
                        }
                    }
                }
            }
        }

        RowLayout {
            Layout.fillWidth: true
            Layout.topMargin: Theme.spacingLarge
            spacing: Theme.spacingMedium
            Item {
                Layout.fillWidth: true
            }
            Button {
                text: root.translator.tr("common.ok")
                highlighted: true
                onClicked: root.applyRequested(true)
            }
            Button {
                text: root.translator.tr("common.cancel")
                onClicked: {
                    root.loadCurrent();
                    root.closeRequested();
                }
            }
            Button {
                text: root.translator.tr("common.apply")
                onClicked: root.applyRequested(false)
            }
        }
    }

}
