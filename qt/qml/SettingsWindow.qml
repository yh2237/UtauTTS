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
    signal saveRequested()

    title: root.translator.tr("settings.title")
    visible: false
    width: 720
    height: 540
    minimumWidth: 580
    minimumHeight: 420
    transientParent: hostWindow
    modality: Qt.ApplicationModal
    flags: Qt.Dialog
    palette: hostPalette
    color: palette.window

    property int currentPage: 0
    property string pendingDefaultVoicebankId: ""
    property string pendingDefaultModelId: "frame-intonation-v8"
    property string pendingDefaultRendererId: "openutau-worldline-r-faithful"
    property int pendingMoraDuration: 120
    property int pendingPauseDuration: 180
    property int pendingLeadingPreutterance: 0
    property bool pendingApplyPitch: true
    property bool pendingExportTextWithWav: false
    property bool pendingExportLabWithWav: false
    property string pendingExportTextEncoding: "utf-8"
    property bool pendingDarkMode: false
    property string pendingLanguage: "ja"
    property var languageCodes: root.backend.languageCodes()
    property bool pendingCloseLogOnSuccess: true
    property bool pendingUpdateCheckEnabled: true
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
        for (let index = 0; index < root.languageCodes.length; ++index)
            labels.push(root.backend.languageDisplayName(root.languageCodes[index]));
        return labels;
    }

    function loadCurrent() {
        pendingDefaultVoicebankId = root.backend.defaultVoicebankId;
        pendingDefaultModelId = root.backend.defaultModelId;
        pendingDefaultRendererId = root.backend.defaultRenderer;
        pendingMoraDuration = root.backend.defaultMoraDuration;
        pendingPauseDuration = root.backend.defaultPauseDuration;
        pendingLeadingPreutterance = root.backend.defaultLeadingPreutterance;
        leadingPreutteranceSpin.value = pendingLeadingPreutterance;
        pendingApplyPitch = root.backend.defaultApplyPitch;
        pendingExportTextWithWav = root.backend.exportTextWithWav;
        pendingExportLabWithWav = root.backend.exportLabWithWav;
        pendingExportTextEncoding = root.backend.exportTextEncoding;
        pendingDarkMode = root.backend.darkMode;
        pendingLanguage = root.backend.language;
        pendingCloseLogOnSuccess = root.backend.closeLogOnSuccess;
        pendingUpdateCheckEnabled = root.backend.updateCheckEnabled;
        pendingPreviewCacheFileCount = root.backend.previewCacheFileCount;
        pendingDeveloperMode = root.backend.developerMode;
        pendingSynthesizeShortcut = root.backend.synthesizeShortcut;
        pendingSaveProjectShortcut = root.backend.saveProjectShortcut;
        pendingReloadVoicebanksShortcut = root.backend.reloadVoicebanksShortcut;
        pendingAddUtteranceShortcut = root.backend.addUtteranceShortcut;
        pendingRemoveUtteranceShortcut = root.backend.removeUtteranceShortcut;
        pendingUndoShortcut = root.backend.undoShortcut;
        pendingRedoShortcut = root.backend.redoShortcut;
        themeCombo.currentIndex = pendingDarkMode ? 1 : 0;
        languageCombo.currentIndex = root.languageCodes.indexOf(pendingLanguage);
        defaultVoicebankCombo.currentIndex = root.defaultVoicebankIndex();
        defaultModelCombo.currentIndex = root.defaultModelIndex();
        defaultRendererCombo.currentIndex = root.defaultRendererIndex();
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
            model: [root.translator.tr("settings.page.synthesis"),
                root.translator.tr("settings.page.export"),
                root.translator.tr("settings.page.appearance"),
                root.translator.tr("settings.page.log"),
                root.translator.tr("settings.page.shortcuts")]
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
                                    textFromValue: value => value + " ms"
                                    Component.onCompleted: refreshTextFormatter()
                                    function refreshTextFormatter() {
                                        const automaticText = root.translator.tr("main.aliasPolicy.auto");
                                        textFromValue = value => value === 0
                                                ? automaticText : value + " ms";
                                        const currentValue = value;
                                        value = currentValue < to ? currentValue + 1 : currentValue - 1;
                                        value = currentValue;
                                    }
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
                                            leadingPreutteranceSpin.value = 0;
                                        }
                                    }
                                    Connections {
                                        target: root.translator
                                        function onTranslationsChanged() {
                                            leadingPreutteranceSpin.refreshTextFormatter();
                                        }
                                    }
                                }
                            }
                            RowLayout {
                                Layout.fillWidth: true
                                Label {
                                    text: root.translator.tr("settings.allowManualPitch")
                                    Layout.fillWidth: true
                                }
                                Switch {
                                    id: applyPitchCheck
                                    Layout.alignment: Qt.AlignVCenter | Qt.AlignRight
                                    checked: root.pendingApplyPitch
                                    onToggled: root.pendingApplyPitch = checked
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
                                    Component.onCompleted: refreshTextFormatter()
                                    function refreshTextFormatter() {
                                        const unit = root.translator.tr("settings.previewCacheFileCount.unit");
                                        textFromValue = value => value + " " + unit;
                                        const currentValue = value;
                                        value = currentValue < to ? currentValue + 1 : currentValue - 1;
                                        value = currentValue;
                                    }
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
                                            previewCacheSpin.refreshTextFormatter();
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
                                checked: root.pendingExportTextWithWav
                                onToggled: root.pendingExportTextWithWav = checked
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            enabled: root.pendingExportTextWithWav
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.exportTextEncoding")
                            }
                            ComboBox {
                                Layout.preferredWidth: 180
                                model: ["UTF-8", "Shift_JIS (CP932)"]
                                currentIndex: root.pendingExportTextEncoding === "shift_jis" ? 1 : 0
                                onActivated: root.pendingExportTextEncoding = currentIndex === 1 ? "shift_jis" : "utf-8"
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.exportLabWithWav")
                            }
                            Switch {
                                checked: root.pendingExportLabWithWav
                                onToggled: root.pendingExportLabWithWav = checked
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
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.updateCheckEnabled")
                            }
                            Switch {
                                Layout.alignment: Qt.AlignVCenter | Qt.AlignRight
                                checked: root.pendingUpdateCheckEnabled
                                onToggled: root.pendingUpdateCheckEnabled = checked
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.developerMode")
                            }
                            Switch {
                                Layout.alignment: Qt.AlignVCenter | Qt.AlignRight
                                checked: root.pendingDeveloperMode
                                onToggled: root.pendingDeveloperMode = checked
                            }
                        }
                    }
                }

                ScrollView {
                    id: logSettingsPage
                    contentWidth: availableWidth

                    ColumnLayout {
                        width: logSettingsPage.availableWidth
                        spacing: 8

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.closeLogOnSuccess")
                            }
                            Switch {
                                Layout.alignment: Qt.AlignVCenter | Qt.AlignRight
                                checked: root.pendingCloseLogOnSuccess
                                onToggled: root.pendingCloseLogOnSuccess = checked
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
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                Layout.fillWidth: true
                                text: root.translator.tr("settings.shortcut.redo")
                            }
                            TextField {
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
                    text: root.translator.tr("common.save")
                    onClicked: root.saveRequested()
                }
            }
        }
    }
}
