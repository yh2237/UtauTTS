pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Dialogs
import QtQuick.Layouts

ApplicationWindow {
    id: root
    objectName: "settingsWindow"
    required property var hostWindow
    required property var hostPalette
    required property var backend
    required property var translator
    property var audioOutputDevices: []
    signal applyRequested(bool closeAfter)

    title: root.translator.tr("settings.title")
    visible: false
    width: 720
    height: 540
    minimumWidth: 720
    maximumWidth: 720
    minimumHeight: 540
    maximumHeight: 540
    transientParent: hostWindow
    modality: Qt.ApplicationModal
    flags: Qt.Dialog
    palette: hostPalette
    color: palette.window

    property int currentPage: 0
    property string pendingDefaultVoicebankId: ""
    property string pendingDefaultModelId: "frame-intonation-tcn-v9.1-t"
    property string pendingDefaultRendererId: "utautts-world-phrase"
    property string pendingDefaultAliasPolicy: "auto"
    property string pendingDefaultTone: "C4"
    property bool pendingExportTextWithWav: false
    property bool pendingExportLabWithWav: false
    property string pendingExportTextEncoding: "utf-8"
    property bool pendingDarkMode: false
    property string pendingLanguage: "auto"
    property string pendingFfmpegPath: ""
    property string pendingAudioOutputDeviceId: ""
    property var languageCodes: root.backend.languageCodes()
    property bool pendingCloseLogOnSuccess: true
    property bool pendingUpdateCheckEnabled: true
    property bool pendingPreReleaseUpdateCheckEnabled: false
    property int pendingPreviewCacheFileCount: 32
    property bool pendingAutoPreviewEnabled: true
    property bool pendingExtendedDetailsVisible: false
    property string pendingSynthesizeShortcut: "Ctrl+Enter"
    property string pendingSaveProjectShortcut: "Ctrl+S"
    property string pendingReloadVoicebanksShortcut: "Ctrl+O"
    property string pendingAddUtteranceShortcut: "Ctrl+D"
    property string pendingRemoveUtteranceShortcut: "Delete"
    property string pendingUndoShortcut: "Ctrl+Z"
    property string pendingRedoShortcut: "Ctrl+Y"
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
        return [root.translator.tr("settings.page.synthesis"),
            root.translator.tr("settings.page.export"),
            root.translator.tr("settings.page.appearance"),
            root.translator.tr("settings.page.shortcuts")];
    }

    function selectedRenderer() {
        const renderers = root.backend.renderers;
        const id = String(root.pendingDefaultRendererId || "");
        for (let index = 0; index < renderers.length; ++index) {
            if (String(renderers[index].id) === id)
                return renderers[index];
        }
        return renderers.length ? renderers[0] : null;
    }

    function rendererSettingGroups(renderer) {
        const groups = [];
        if (!renderer || !renderer.settings)
            return groups;
        const settings = renderer.settings;
        for (let index = 0; index < settings.length; ++index) {
            const setting = settings[index];
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

    function loadCurrent() {
        pendingDefaultVoicebankId = root.validDefaultVoicebankId(root.backend.defaultVoicebankId);
        pendingDefaultModelId = root.validDefaultModelId(root.backend.defaultModelId);
        pendingDefaultRendererId = root.validDefaultRendererId(root.backend.defaultRenderer);
        pendingDefaultAliasPolicy = root.backend.defaultAliasPolicy;
        pendingDefaultTone = root.backend.defaultTone;
        pendingExportTextWithWav = root.backend.exportTextWithWav;
        pendingExportLabWithWav = root.backend.exportLabWithWav;
        pendingExportTextEncoding = root.backend.exportTextEncoding;
        pendingDarkMode = root.backend.darkMode;
        pendingLanguage = root.backend.language;
        pendingFfmpegPath = root.backend.ffmpegPath;
        pendingAudioOutputDeviceId = root.backend.audioOutputDeviceId;
        pendingCloseLogOnSuccess = root.backend.closeLogOnSuccess;
        pendingUpdateCheckEnabled = root.backend.updateCheckEnabled;
        pendingPreReleaseUpdateCheckEnabled = root.backend.preReleaseUpdateCheckEnabled;
        pendingPreviewCacheFileCount = root.backend.previewCacheFileCount;
        pendingAutoPreviewEnabled = root.backend.autoPreviewEnabled;
        pendingExtendedDetailsVisible = root.backend.extendedDetailsVisible;
        pendingSynthesizeShortcut = root.backend.synthesizeShortcut;
        pendingSaveProjectShortcut = root.backend.saveProjectShortcut;
        pendingReloadVoicebanksShortcut = root.backend.reloadVoicebanksShortcut;
        pendingAddUtteranceShortcut = root.backend.addUtteranceShortcut;
        pendingRemoveUtteranceShortcut = root.backend.removeUtteranceShortcut;
        pendingUndoShortcut = root.backend.undoShortcut;
        pendingRedoShortcut = root.backend.redoShortcut;
    }

    function resetDefaultVoicebank() {
        pendingDefaultVoicebankId = "";
    }

    function resetDefaultAliasPolicy() {
        pendingDefaultAliasPolicy = "auto";
    }

    function resetDefaultModel() {
        pendingDefaultModelId = root.validDefaultModelId("frame-intonation-tcn-v9.1-t");
    }

    function resetDefaultTone() {
        pendingDefaultTone = "C4";
    }

    function resetPreviewCacheFileCount() {
        pendingPreviewCacheFileCount = 32;
    }

    function resetExportTextWithWav() {
        pendingExportTextWithWav = false;
    }

    function resetExportTextEncoding() {
        pendingExportTextEncoding = "utf-8";
    }

    function resetExportLabWithWav() {
        pendingExportLabWithWav = false;
    }

    function resetTheme() {
        pendingDarkMode = false;
    }

    function resetLanguage() {
        pendingLanguage = "auto";
    }

    function resetFfmpegPath() {
        pendingFfmpegPath = "";
    }

    function resetAudioOutputDevice() {
        pendingAudioOutputDeviceId = "";
    }

    function resetUpdateCheckEnabled() {
        pendingUpdateCheckEnabled = true;
    }

    function resetPreReleaseUpdateCheckEnabled() {
        pendingPreReleaseUpdateCheckEnabled = false;
    }

    function resetAutoPreviewEnabled() {
        pendingAutoPreviewEnabled = true;
    }

    function resetExtendedDetailsVisible() {
        pendingExtendedDetailsVisible = false;
    }

    function resetCloseLogOnSuccess() {
        pendingCloseLogOnSuccess = true;
    }

    function resetSynthesizeShortcut() {
        pendingSynthesizeShortcut = "Ctrl+Enter";
    }

    function resetSaveProjectShortcut() {
        pendingSaveProjectShortcut = "Ctrl+S";
    }

    function resetReloadVoicebanksShortcut() {
        pendingReloadVoicebanksShortcut = "Ctrl+O";
    }

    function resetAddUtteranceShortcut() {
        pendingAddUtteranceShortcut = "Ctrl+D";
    }

    function resetRemoveUtteranceShortcut() {
        pendingRemoveUtteranceShortcut = "Delete";
    }

    function resetUndoShortcut() {
        pendingUndoShortcut = "Ctrl+Z";
    }

    function resetRedoShortcut() {
        pendingRedoShortcut = "Ctrl+Y";
    }

    function defaultVoicebankIndex() {
        const id = String(root.pendingDefaultVoicebankId || "");
        if (!id.length)
            return 0;
        for (let index = 0; index < root.backend.voicebanks.length; ++index) {
            if (root.backend.voicebanks[index].id === id)
                return index + 1;
        }
        return 0;
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

    function validDefaultRendererId(value) {
        const id = String(value || "");
        for (let index = 0; index < root.backend.renderers.length; ++index)
            if (root.backend.renderers[index].id === id)
                return id;
        return root.backend.renderers.length ? root.backend.renderers[0].id : "";
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

    RowLayout {
        anchors.fill: parent
        anchors.margins: 12
        spacing: 12

        ListView {
            id: settingsNavigation
            Layout.preferredWidth: 170
            Layout.fillHeight: true
            clip: true
            model: root.settingsPageLabels()
            currentIndex: root.currentPage

            delegate: ItemDelegate {
                required property int index
                required property string modelData
                width: ListView.view.width
                text: modelData
                highlighted: ListView.isCurrentItem
                onClicked: root.currentPage = index
            }
        }

        Rectangle {
            Layout.preferredWidth: 1
            Layout.fillHeight: true
            color: root.hostWindow.borderColor
        }

        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 10

            StackLayout {
                Layout.fillWidth: true
                Layout.fillHeight: true
                currentIndex: root.currentPage

                Item {
                    id: timingSettingsPage

                    ColumnLayout {
                        anchors.fill: parent
                        spacing: 8

                        Item {
                            id: rendererTabsHeader
                            Layout.fillWidth: true
                            Layout.preferredHeight: 44

                            Rectangle {
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.top: rendererTabFlick.bottom
                                height: 1
                                color: root.hostWindow.borderColor
                            }

                            Flickable {
                                id: rendererTabFlick
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.top: parent.top
                                height: 35
                                contentWidth: rendererTabRow.width
                                contentHeight: height
                                clip: true
                                boundsBehavior: Flickable.StopAtBounds

                                WheelHandler {
                                    onWheel: event => {
                                        const maximum = Math.max(0, rendererTabFlick.contentWidth - rendererTabFlick.width);
                                        rendererTabFlick.contentX = Math.max(0, Math.min(maximum, rendererTabFlick.contentX - event.angleDelta.y));
                                        event.accepted = true;
                                    }
                                }

                                Row {
                                    id: rendererTabRow
                                    height: rendererTabFlick.height
                                    spacing: 0

                                    Repeater {
                                        id: rendererTabRepeater
                                        model: root.backend.renderers

                                        ToolButton {
                                            id: rendererTab
                                            required property int index
                                            required property var modelData
                                            width: Math.max(96, rendererTabLabel.implicitWidth + 24)
                                            height: rendererTabRow.height
                                            ButtonGroup.group: rendererTabGroup
                                            checkable: true
                                            checked: String(modelData.id) === String(root.pendingDefaultRendererId)
                                            text: String(modelData.display_name || modelData.id)
                                            onClicked: root.pendingDefaultRendererId = String(modelData.id)

                                            background: Rectangle {
                                                color: rendererTab.checked
                                                       ? root.palette.base
                                                       : rendererTab.hovered
                                                         ? Qt.rgba(root.palette.alternateBase.r,
                                                                   root.palette.alternateBase.g,
                                                                   root.palette.alternateBase.b, 0.42)
                                                         : "transparent"
                                                Rectangle {
                                                    anchors.left: parent.left
                                                    anchors.right: parent.right
                                                    anchors.bottom: parent.bottom
                                                    height: 1
                                                    color: rendererTab.checked ? root.palette.base : "transparent"
                                                }
                                                Rectangle {
                                                    visible: rendererTab.index < rendererTabRepeater.count - 1
                                                    anchors.right: parent.right
                                                    anchors.verticalCenter: parent.verticalCenter
                                                    width: 1
                                                    height: 18
                                                    color: root.hostWindow.borderColor
                                                }
                                            }
                                            contentItem: Text {
                                                id: rendererTabLabel
                                                anchors.centerIn: parent
                                                text: rendererTab.text
                                                color: rendererTab.checked
                                                       ? root.palette.text : root.palette.placeholderText
                                                font.pixelSize: 13
                                                horizontalAlignment: Text.AlignHCenter
                                                verticalAlignment: Text.AlignVCenter
                                            }
                                        }
                                    }
                                }
                            }

                            ScrollBar {
                                id: rendererTabBar
                                orientation: Qt.Horizontal
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.top: rendererTabFlick.bottom
                                anchors.topMargin: 2
                                height: 7
                                policy: ScrollBar.AsNeeded
                                size: rendererTabFlick.visibleArea.widthRatio
                                position: rendererTabFlick.visibleArea.xPosition
                                active: rendererTabFlick.movingHorizontally || hovered
                                onPositionChanged: rendererTabFlick.contentX = position * Math.max(0, rendererTabFlick.contentWidth - rendererTabFlick.width)
                            }

                            ButtonGroup {
                                id: rendererTabGroup
                                exclusive: true
                            }
                        }

                        ScrollView {
                            id: synthesisSettingsPage
                            Layout.fillWidth: true
                            Layout.fillHeight: true
                            contentWidth: availableWidth

                            ColumnLayout {
                                width: synthesisSettingsPage.availableWidth
                                spacing: 14

                                ColumnLayout {
                                    Layout.fillWidth: true
                                    spacing: 8

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
                                                Layout.preferredWidth: 240
                                                model: [{
                                                    id: "",
                                                    name: root.translator.tr("settings.defaultVoicebank.auto")
                                                }].concat(root.backend.voicebanks)
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
                                                text: root.translator.tr("settings.defaultAliasPolicy")
                                                Layout.fillWidth: true
                                            }
                                            ComboBox {
                                                id: defaultAliasPolicyCombo
                                                Layout.preferredWidth: 240
                                                model: [
                                                    { id: "auto", display_name: root.translator.tr("main.aliasPolicy.auto") },
                                                    { id: "cvvc-enhanced", display_name: root.translator.tr("main.aliasPolicy.cvvcEnhanced") },
                                                    { id: "vcv-prefer", display_name: root.translator.tr("main.aliasPolicy.vcvPrefer") },
                                                    { id: "cvvc-prefer", display_name: root.translator.tr("main.aliasPolicy.cvvcPrefer") },
                                                    { id: "cv-only", display_name: root.translator.tr("main.aliasPolicy.cvOnly") }
                                                ]
                                                textRole: "display_name"
                                                valueRole: "id"
                                                currentIndex: indexOfValue(root.pendingDefaultAliasPolicy)
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
                                                Layout.preferredWidth: 240
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
                                        RowLayout {
                                            Layout.fillWidth: true
                                            Label {
                                                text: root.translator.tr("settings.defaultTone")
                                                Layout.fillWidth: true
                                            }
                                            TextField {
                                                id: defaultToneField
                                                Layout.preferredWidth: 180
                                                horizontalAlignment: TextInput.AlignRight
                                                text: root.pendingDefaultTone
                                                onEditingFinished: root.pendingDefaultTone = text.trim().length ? text.trim() : "C4"
                                            }
                                            SettingsResetButton {
                                                translator: root.translator
                                                onResetRequested: root.resetDefaultTone()
                                            }
                                        }
                                    }
                                    Repeater {
                                        model: root.rendererSettingGroups(root.selectedRenderer())
                                        delegate: ColumnLayout {
                                            id: rendererGroup
                                            required property var modelData
                                            Layout.fillWidth: true
                                            spacing: 8

                                            Label {
                                                Layout.fillWidth: true
                                                text: root.groupLabel(String(rendererGroup.modelData.id))
                                                font.bold: true
                                            }

                                            Repeater {
                                                model: rendererGroup.modelData.settings
                                                delegate: RowLayout {
                                                    id: rendererSettingRow
                                                    required property var modelData
                                                    Layout.fillWidth: true
                                                    property string rendererId: String(rendererGroup.modelData.id)
                                                    property string settingType: String(rendererSettingRow.modelData.type || "integer")
                                                    property bool scaled: rendererSettingRow.settingType === "number"
                                                    property real factor: rendererSettingRow.scaled ? 100 : 1

                                                function settingFallback() {
                                                    return rendererSettingRow.modelData.default;
                                                }
                                                function storedValue() {
                                                    const map = root.backend.rendererSettings;
                                                    const key = rendererSettingRow.rendererId + "/"
                                                            + String(rendererSettingRow.modelData.id);
                                                    if (map && map[key] !== undefined)
                                                        return map[key];
                                                    return rendererSettingRow.settingFallback();
                                                }
                                                function minimum() {
                                                    return rendererSettingRow.modelData.min !== undefined
                                                            ? Number(rendererSettingRow.modelData.min) : 0;
                                                }
                                                function maximum() {
                                                    if (rendererSettingRow.modelData.max !== undefined)
                                                        return Number(rendererSettingRow.modelData.max);
                                                    return rendererSettingRow.scaled ? 1 : 100;
                                                }
                                                function stepSize() {
                                                    const step = rendererSettingRow.modelData.step !== undefined
                                                            ? Number(rendererSettingRow.modelData.step) : 1;
                                                    const scaledStep = step * rendererSettingRow.factor;
                                                    return scaledStep > 0 ? scaledStep : 1;
                                                }
                                                function scaledValue() {
                                                    return Math.round(Number(rendererSettingRow.storedValue())
                                                                      * rendererSettingRow.factor);
                                                }
                                                function actualValue(value) {
                                                    return rendererSettingRow.scaled
                                                            ? value / rendererSettingRow.factor : value;
                                                }
                                                function textValue(value) {
                                                    return rendererSettingRow.scaled
                                                            ? (value / rendererSettingRow.factor).toFixed(2)
                                                            : String(value);
                                                }
                                                function parsedValue(text) {
                                                    const parsed = rendererSettingRow.scaled
                                                            ? parseFloat(text) : parseInt(text);
                                                    return isNaN(parsed)
                                                            ? 0
                                                            : Math.round(parsed * rendererSettingRow.factor);
                                                }
                                                function applyValue(value) {
                                                    root.backend.setRendererSetting(
                                                        rendererSettingRow.rendererId,
                                                        String(rendererSettingRow.modelData.id),
                                                        rendererSettingRow.actualValue(value));
                                                }
                                                function enumModel() {
                                                    const source = String(rendererSettingRow.modelData.options_source || "");
                                                    if (source === "resamplers" || source === "wavtools") {
                                                        const tools = source === "resamplers"
                                                                      ? root.backend.resamplers
                                                                      : root.backend.wavtools;
                                                        const rows = [];
                                                        if (source === "resamplers")
                                                            rows.push({ value: "", label: root.translator.tr("settings.resamplerAuto") });
                                                        for (let index = 0; index < tools.length; ++index) {
                                                            const tool = tools[index];
                                                            rows.push({
                                                                value: String(tool.id),
                                                                label: String(tool.display_name || tool.id)
                                                            });
                                                        }
                                                        return rows;
                                                    }
                                                    const options = rendererSettingRow.modelData.options || [];
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
                                                              rendererSettingRow.modelData.label
                                                              || rendererSettingRow.modelData.id)
                                                }
                                                SpinBox {
                                                    visible: rendererSettingRow.settingType === "integer"
                                                             || rendererSettingRow.settingType === "number"
                                                    Layout.preferredWidth: 180
                                                    Layout.alignment: Qt.AlignVCenter
                                                    from: Math.round(rendererSettingRow.minimum()
                                                                     * rendererSettingRow.factor)
                                                    to: Math.round(rendererSettingRow.maximum()
                                                                   * rendererSettingRow.factor)
                                                    stepSize: rendererSettingRow.stepSize()
                                                    value: rendererSettingRow.scaledValue()
                                                    editable: true
                                                    textFromValue: value => rendererSettingRow.textValue(value)
                                                    valueFromText: text => rendererSettingRow.parsedValue(text)
                                                    onValueModified: rendererSettingRow.applyValue(value)
                                                }
                                                Switch {
                                                    visible: rendererSettingRow.settingType === "boolean"
                                                    Layout.alignment: Qt.AlignVCenter
                                                    checked: !!rendererSettingRow.storedValue()
                                                    onToggled: root.backend.setRendererSetting(
                                                        rendererSettingRow.rendererId,
                                                        String(rendererSettingRow.modelData.id),
                                                        checked)
                                                }
                                                ComboBox {
                                                    visible: rendererSettingRow.settingType === "enum"
                                                    Layout.preferredWidth: 240
                                                    model: rendererSettingRow.enumModel()
                                                    textRole: "label"
                                                    valueRole: "value"
                                                    currentIndex: indexOfValue(String(rendererSettingRow.storedValue()))
                                                    onActivated: root.backend.setRendererSetting(
                                                        rendererSettingRow.rendererId,
                                                        String(rendererSettingRow.modelData.id),
                                                        currentValue)
                                                }
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                }

                ScrollView {
                    id: exportSettingsPage
                    contentWidth: availableWidth

                    ColumnLayout {
                        width: exportSettingsPage.availableWidth
                        spacing: 12

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.exportTextWithWav")
                            }
                            Switch {
                                id: exportTextWithWavSwitch
                                checked: root.pendingExportTextWithWav
                                onToggled: root.pendingExportTextWithWav = checked
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetExportTextWithWav()
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
                                Layout.preferredWidth: 180
                                enabled: root.pendingExportTextWithWav
                                model: ["UTF-8", "Shift_JIS (CP932)"]
                                currentIndex: root.pendingExportTextEncoding === "shift_jis" ? 1 : 0
                                onActivated: root.pendingExportTextEncoding = currentIndex === 1 ? "shift_jis" : "utf-8"
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetExportTextEncoding()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.exportLabWithWav")
                            }
                            Switch {
                                id: exportLabWithWavSwitch
                                checked: root.pendingExportLabWithWav
                                onToggled: root.pendingExportLabWithWav = checked
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetExportLabWithWav()
                            }
                        }
                    }
                }

                ScrollView {
                    id: appearanceSettingsPage
                    contentWidth: availableWidth

                    ColumnLayout {
                        width: appearanceSettingsPage.availableWidth
                        spacing: 8

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.theme")
                            }
                            ComboBox {
                                id: themeCombo
                                Layout.preferredWidth: 180
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
                                Layout.preferredWidth: 180
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
                                Layout.preferredWidth: 300
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
                                Layout.preferredWidth: 180
                                from: 1
                                to: 256
                                value: root.pendingPreviewCacheFileCount
                                editable: true
                                property string unitText: ""
                                textFromValue: value => value + " " + previewCacheSpin.unitText
                                function refreshText() {
                                    unitText = root.translator.tr("settings.previewCacheFileCount.unit");
                                    Qt.callLater(() => contentItem.text = textFromValue(value, locale));
                                }
                                Component.onCompleted: refreshText()
                                valueFromText: text => parseInt(text)
                                onValueModified: root.pendingPreviewCacheFileCount = value
                                TapHandler {
                                    acceptedButtons: Qt.LeftButton
                                    grabPermissions: PointerHandler.CanTakeOverFromAnything
                                    onDoubleTapped: root.pendingPreviewCacheFileCount = 32
                                }
                                Connections {
                                    target: root.translator
                                    function onTranslationsChanged() {
                                        previewCacheSpin.refreshText();
                                    }
                                }
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetPreviewCacheFileCount()
                            }
                        }

                    }
                }

                ScrollView {
                    id: shortcutSettingsPage
                    contentWidth: availableWidth

                    ColumnLayout {
                        width: shortcutSettingsPage.availableWidth
                        spacing: 12

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
                                Layout.preferredWidth: 180
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
                                Layout.preferredWidth: 180
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
                                Layout.preferredWidth: 180
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
                                Layout.preferredWidth: 180
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
                                Layout.preferredWidth: 180
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
                                Layout.preferredWidth: 180
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
                                Layout.preferredWidth: 180
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

            }

            RowLayout {
                Layout.fillWidth: true
                Item {
                    Layout.fillWidth: true
                }
                Button {
                    text: root.translator.tr("common.ok")
                    onClicked: root.applyRequested(true)
                }
                Button {
                    text: root.translator.tr("common.cancel")
                    onClicked: {
                        root.loadCurrent();
                        root.close();
                    }
                }
                Button {
                    text: root.translator.tr("common.apply")
                    onClicked: root.applyRequested(false)
                }
            }
        }
    }

}
