pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ApplicationWindow {
    id: root
    objectName: "settingsWindow"
    required property var hostWindow
    required property var hostPalette
    required property var backend
    required property var translator
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
    property string pendingDefaultModelId: "frame-intonation-v8"
    property string pendingDefaultRendererId: "utautts-world-phrase"
    property string pendingDefaultAliasPolicy: "auto"
    property string pendingDefaultTone: "C4"
    property int pendingMoraDuration: 120
    property int pendingPauseDuration: 180
    property int pendingLeadingPreutterance: 0
    property real pendingDefaultIntonationStrength: 2.0
    property bool pendingExportTextWithWav: false
    property bool pendingExportLabWithWav: false
    property string pendingExportTextEncoding: "utf-8"
    property bool pendingDarkMode: false
    property string pendingLanguage: "auto"
    property var languageCodes: root.backend.languageCodes()
    property bool pendingCloseLogOnSuccess: true
    property bool pendingUpdateCheckEnabled: true
    property bool pendingPreReleaseUpdateCheckEnabled: false
    property int pendingPreviewCacheFileCount: 32
    property bool pendingDeveloperMode: false
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
        const pages = [root.translator.tr("settings.page.synthesis"),
            root.translator.tr("settings.page.export"),
            root.translator.tr("settings.page.appearance"),
            root.translator.tr("settings.page.shortcuts")];
        if (root.backend.developerMode)
            pages.push(root.translator.tr("settings.page.developer"));
        return pages;
    }

    Connections {
        target: root.backend
        function onDeveloperModeChanged() {
            if (!root.backend.developerMode && root.currentPage >= 4)
                root.currentPage = 2;
        }
    }

    function loadCurrent() {
        pendingDefaultVoicebankId = root.validDefaultVoicebankId(root.backend.defaultVoicebankId);
        pendingDefaultModelId = root.validDefaultModelId(root.backend.defaultModelId);
        pendingDefaultRendererId = root.validDefaultRendererId(root.backend.defaultRenderer);
        pendingDefaultAliasPolicy = root.backend.defaultAliasPolicy;
        pendingDefaultTone = root.backend.defaultTone;
        pendingMoraDuration = root.backend.defaultMoraDuration;
        pendingPauseDuration = root.backend.defaultPauseDuration;
        pendingLeadingPreutterance = root.backend.defaultLeadingPreutterance;
        pendingDefaultIntonationStrength = root.backend.defaultIntonationStrength;
        pendingExportTextWithWav = root.backend.exportTextWithWav;
        pendingExportLabWithWav = root.backend.exportLabWithWav;
        pendingExportTextEncoding = root.backend.exportTextEncoding;
        pendingDarkMode = root.backend.darkMode;
        pendingLanguage = root.backend.language;
        pendingCloseLogOnSuccess = root.backend.closeLogOnSuccess;
        pendingUpdateCheckEnabled = root.backend.updateCheckEnabled;
        pendingPreReleaseUpdateCheckEnabled = root.backend.preReleaseUpdateCheckEnabled;
        pendingPreviewCacheFileCount = root.backend.previewCacheFileCount;
        pendingDeveloperMode = root.backend.developerMode;
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
        pendingDefaultModelId = root.validDefaultModelId("frame-intonation-v8");
    }

    function resetDefaultRenderer() {
        pendingDefaultRendererId = root.validDefaultRendererId("utautts-world-phrase");
    }

    function resetDefaultTone() {
        pendingDefaultTone = "C4";
    }

    function resetDefaultIntonation() {
        pendingDefaultIntonationStrength = 2.0;
    }

    function resetDefaultMoraDuration() {
        pendingMoraDuration = 120;
    }

    function resetDefaultPauseDuration() {
        pendingPauseDuration = 180;
    }

    function resetDefaultLeadingPreutterance() {
        pendingLeadingPreutterance = 0;
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

    function resetUpdateCheckEnabled() {
        pendingUpdateCheckEnabled = true;
    }

    function resetPreReleaseUpdateCheckEnabled() {
        pendingPreReleaseUpdateCheckEnabled = false;
    }

    function resetDeveloperMode() {
        pendingDeveloperMode = false;
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

    function defaultRendererIndex() {
        for (let index = 0; index < root.backend.renderers.length; ++index)
            if (root.backend.renderers[index].id === root.pendingDefaultRendererId)
                return index;
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

                ScrollView {
                    id: timingSettingsPage
                    contentWidth: availableWidth

                    ColumnLayout {
                        width: timingSettingsPage.availableWidth
                        spacing: 14

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 8

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
                                    text: root.translator.tr("settings.defaultIntonation")
                                    Layout.fillWidth: true
                                }
                                SpinBox {
                                    id: defaultIntonationSpin
                                    Layout.preferredWidth: 180
                                    from: 0
                                    to: 400
                                    stepSize: 5
                                    value: Math.round(root.pendingDefaultIntonationStrength * 100)
                                    editable: true
                                    textFromValue: value => (value / 100).toFixed(2)
                                    valueFromText: text => Math.round(parseFloat(text) * 100)
                                    onValueModified: root.pendingDefaultIntonationStrength = value / 100
                                    TapHandler {
                                        acceptedButtons: Qt.LeftButton
                                        grabPermissions: PointerHandler.CanTakeOverFromAnything
                                        onDoubleTapped: root.pendingDefaultIntonationStrength = 2.0
                                    }
                                }
                                SettingsResetButton {
                                    translator: root.translator
                                    onResetRequested: root.resetDefaultIntonation()
                                }
                            }
                            RowLayout {
                                Layout.fillWidth: true
                                Label {
                                    text: root.translator.tr("settings.defaultMoraDuration")
                                    Layout.fillWidth: true
                                }
                                SpinBox {
                                    id: moraSpin
                                    Layout.preferredWidth: 180
                                    Layout.alignment: Qt.AlignVCenter
                                    from: 20
                                    to: 1000
                                    value: root.pendingMoraDuration
                                    editable: true
                                    textFromValue: value => value + " ms"
                                    valueFromText: text => parseInt(text)
                                    onValueModified: root.pendingMoraDuration = value
                                    TapHandler {
                                        acceptedButtons: Qt.LeftButton
                                        grabPermissions: PointerHandler.CanTakeOverFromAnything
                                        onDoubleTapped: root.pendingMoraDuration = 120
                                    }
                                }
                                SettingsResetButton {
                                    translator: root.translator
                                    onResetRequested: root.resetDefaultMoraDuration()
                                }
                            }
                            RowLayout {
                                Layout.fillWidth: true
                                Label {
                                    text: root.translator.tr("settings.defaultPauseDuration")
                                    Layout.fillWidth: true
                                }
                                SpinBox {
                                    id: pauseSpin
                                    Layout.preferredWidth: 180
                                    Layout.alignment: Qt.AlignVCenter
                                    from: 0
                                    to: 3000
                                    value: root.pendingPauseDuration
                                    editable: true
                                    textFromValue: value => value + " ms"
                                    valueFromText: text => parseInt(text)
                                    onValueModified: root.pendingPauseDuration = value
                                    TapHandler {
                                        acceptedButtons: Qt.LeftButton
                                        grabPermissions: PointerHandler.CanTakeOverFromAnything
                                        onDoubleTapped: root.pendingPauseDuration = 180
                                    }
                                }
                                SettingsResetButton {
                                    translator: root.translator
                                    onResetRequested: root.resetDefaultPauseDuration()
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
                                    text: root.translator.tr("settings.defaultRenderer")
                                    Layout.fillWidth: true
                                }
                                ComboBox {
                                    id: defaultRendererCombo
                                    Layout.preferredWidth: 240
                                    model: root.backend.renderers
                                    textRole: "display_name"
                                    valueRole: "id"
                                    currentIndex: root.defaultRendererIndex()
                                    onActivated: root.pendingDefaultRendererId = currentValue
                                }
                                SettingsResetButton {
                                    translator: root.translator
                                    onResetRequested: root.resetDefaultRenderer()
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
                            RowLayout {
                                Layout.fillWidth: true
                                Label {
                                    text: root.translator.tr("settings.defaultLeadingPreutterance")
                                    Layout.fillWidth: true
                                }
                                SpinBox {
                                    id: leadingPreutteranceSpin
                                    Layout.preferredWidth: 180
                                    Layout.alignment: Qt.AlignVCenter
                                    from: 0
                                    to: 300
                                    stepSize: 5
                                    value: root.pendingLeadingPreutterance
                                    editable: true
                                    property string automaticText: ""
                                    textFromValue: value => value === 0
                                            ? leadingPreutteranceSpin.automaticText : value + " ms"
                                    function refreshText() {
                                        automaticText = root.translator.tr("main.aliasPolicy.auto");
                                        Qt.callLater(() => contentItem.text = textFromValue(value, locale));
                                    }
                                    Component.onCompleted: refreshText()
                                    valueFromText: text => {
                                        const parsed = parseInt(text);
                                        return isNaN(parsed) ? 0 : parsed;
                                    }
                                    onValueModified: root.pendingLeadingPreutterance = value
                                    TapHandler {
                                        acceptedButtons: Qt.LeftButton
                                        grabPermissions: PointerHandler.CanTakeOverFromAnything
                                        onDoubleTapped: {
                                            root.pendingLeadingPreutterance = 0;
                                        }
                                    }
                                    Connections {
                                        target: root.translator
                                        function onTranslationsChanged() {
                                            leadingPreutteranceSpin.refreshText();
                                        }
                                    }
                                }
                                SettingsResetButton {
                                    translator: root.translator
                                    onResetRequested: root.resetDefaultLeadingPreutterance()
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

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.developerMode")
                            }
                            Switch {
                                id: developerModeSwitch
                                Layout.alignment: Qt.AlignVCenter | Qt.AlignRight
                                checked: root.pendingDeveloperMode
                                onToggled: root.pendingDeveloperMode = checked
                            }
                            SettingsResetButton {
                                translator: root.translator
                                onResetRequested: root.resetDeveloperMode()
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

                ScrollView {
                    id: developerSettingsPage
                    visible: root.backend.developerMode
                    contentWidth: availableWidth

                    ColumnLayout {
                        width: developerSettingsPage.availableWidth
                        spacing: 8

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
