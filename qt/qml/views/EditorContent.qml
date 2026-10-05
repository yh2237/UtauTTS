pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import UtauTTS.Media
    SplitView {
        id: editorSplitView
        required property var window
        readonly property bool mobileLayout: window.mobileLayout
        readonly property bool extendedPitchEditorVisible: pitchModeTabs.currentIndex === 1

        property alias pitchEditor: pitchEditor
        property alias phonemeEditor: phonemeEditor
        property alias utteranceList: utteranceList
        property alias voiceCombo: voiceCombo
        property alias speechLanguageCombo: speechLanguageCombo
        property alias phonemizerCombo: phonemizerCombo
        property alias aliasPolicyCombo: aliasPolicyCombo
        property alias modelCombo: modelCombo
        property alias rendererCombo: rendererCombo
        property alias toneField: toneField
        property alias colorCombo: colorCombo
        property alias intonationInput: intonationInput
        property alias intonationSlider: intonationSlider
        property alias moraInput: moraInput
        property alias moraSlider: moraSlider
        property alias pauseInput: pauseInput
        property alias pauseSlider: pauseSlider
        property alias leadingPreutteranceInput: leadingPreutteranceInput
        property alias leadingPreutteranceSlider: leadingPreutteranceSlider

        FontLoader {
            id: iconFont
            source: "qrc:/fonts/MaterialSymbolsOutlined-subset.ttf"
        }

        Keys.onPressed: event => {
            if (event.key === Qt.Key_Shift && !event.isAutoRepeat) {
                pitchEditor.setShiftPreview(true);
                phonemeEditor.setShiftPreview(true);
            }
        }
        Keys.onReleased: event => {
            if (event.key === Qt.Key_Shift && !event.isAutoRepeat) {
                pitchEditor.setShiftPreview(false);
                phonemeEditor.setShiftPreview(false);
            }
        }

        anchors.fill: parent
        orientation: Qt.Vertical
        handle: Item {
            implicitWidth: editorSplitView.width
            implicitHeight: editorSplitView.mobileLayout ? 0 : 10

            Rectangle {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                height: 1
                color: window.borderColor
            }
        }

        SplitView {
            id: utteranceSplitView
            SplitView.fillHeight: !window.intonationLab
            SplitView.preferredHeight: window.intonationLab ? 66 : -1
            SplitView.minimumHeight: window.intonationLab ? 66 : 0
            SplitView.maximumHeight: window.intonationLab ? 66 : 16777215
            orientation: Qt.Horizontal

            Pane {
                SplitView.fillWidth: true
                SplitView.minimumWidth: window.intonationLab || editorSplitView.mobileLayout
                        ? 0 : Math.min(560, Math.max(0, editorSplitView.width - 280))
                padding: editorSplitView.mobileLayout ? 12 : 10
                background: Rectangle {
                    color: window.palette.window
                }

                ListView {
                    id: utteranceList
                    anchors.fill: parent
                    model: window.utterancesModel
                    clip: true
                    spacing: 4
                    boundsBehavior: Flickable.StopAtBounds
                    bottomMargin: window.intonationLab ? 0 : 64
                    ScrollBar.vertical: ScrollBar {
                        id: utteranceScrollBar
                        parent: editorSplitView.mobileLayout ? utteranceList.parent : utteranceList
                        width: editorSplitView.mobileLayout ? 8 : implicitWidth
                        anchors.right: parent.right
                        anchors.rightMargin: editorSplitView.mobileLayout ? -10 : 0
                        anchors.top: parent.top
                        anchors.bottom: parent.bottom
                        policy: window.intonationLab ? ScrollBar.AlwaysOff : ScrollBar.AlwaysOn
                    }

                    delegate: Item {
                        id: card
                        required property int index
                        required property string content
                        required property string voicebankId
                        required property string imagePath
                        property alias textEditor: utteranceEditor

                        visible: !window.intonationLab || card.index === window.selectedIndex
                        width: Math.max(0, utteranceList.width - (editorSplitView.mobileLayout ? 0 : 16))
                        height: visible ? 46 : 0

                        RowLayout {
                            anchors.fill: parent
                            spacing: 6

                            Rectangle {
                                id: imageHandle
                                visible: !window.intonationLab
                                Layout.preferredWidth: 42
                                Layout.preferredHeight: 42
                                radius: 2
                                color: window.palette.alternateBase
                                border.color: card.index === window.selectedIndex ? window.accent : window.borderColor

                                Image {
                                    anchors.fill: parent
                                    anchors.margins: 2
                                    source: window.localImageUrl(card.imagePath)
                                    fillMode: Image.PreserveAspectFit
                                    asynchronous: true
                                }
                                Label {
                                    anchors.centerIn: parent
                                    visible: !card.imagePath
                                    text: window.translator.tr("main.card.icon")
                                    color: window.mutedText
                                    font.pixelSize: 9
                                }

                                DragHandler {
                                    id: imageDrag
                                    enabled: !window.batchExportActive
                                    target: dragProxy
                                    onActiveChanged: {
                                        if (active) {
                                            window.selectUtterance(card.index);
                                            window.draggedUtteranceIndex = card.index;
                                            dragProxy.x = imageHandle.x;
                                            dragProxy.y = imageHandle.y;
                                        } else
                                            window.draggedUtteranceIndex = -1;
                                    }
                                }
                                ToolTip.visible: imageHover.hovered && !imageDrag.active
                                ToolTip.text: window.voicebankName(card.voicebankId) + "\n" + window.translator.tr("main.card.dragReorder")
                                HoverHandler {
                                    id: imageHover
                                }
                            }

                            TextField {
                                id: utteranceEditor
                                objectName: "utteranceText" + card.index
                                Accessible.name: window.translator.tr("main.textPlaceholder")
                                readOnly: window.intonationLab || window.batchExportActive
                                Layout.fillWidth: true
                                Layout.preferredHeight: 42
                                text: card.content
                                font.pixelSize: 16
                                placeholderText: window.translator.tr("main.textPlaceholder")
                                selectByMouse: true

                                onActiveFocusChanged: {
                                    if (activeFocus)
                                        window.selectUtterance(card.index);
                                }
                                Keys.priority: Keys.BeforeItem
                                Keys.onPressed: event => {
                                    if (event.key === Qt.Key_Delete
                                            && event.modifiers === Qt.NoModifier
                                            && window.qtShortcutSequence(window.appBackend.removeUtteranceShortcut).toLowerCase() === "delete"
                                            && !window.settingsWindowVisible
                                            && !window.appBackend.busy
                                            && !window.batchExportActive
                                            && !window.playbackQueueActive) {
                                        event.accepted = true;
                                        window.removeUtterance();
                                    }
                                }
                                onTextChanged: {
                                    if (card.index >= window.utterancesModel.count || window.utterancesModel.get(card.index).content === text)
                                        return;
                                    window.updateUtteranceText(card.index, text);
                                }
                            }

                            ToolButton {
                                id: cardMenuButton
                                enabled: !window.batchExportActive
                                contentItem: Text {
                                    anchors.centerIn: parent
                                    width: 22
                                    height: 22
                                    text: editorSplitView.mobileLayout ? "\ue3c9" : "\ue5d4"
                                    color: cardMenuButton.palette.buttonText
                                    font.family: iconFont.name
                                    font.pixelSize: 20
                                    horizontalAlignment: Text.AlignHCenter
                                    verticalAlignment: Text.AlignVCenter
                                }
                                visible: !window.intonationLab && (editorSplitView.mobileLayout || card.index === window.selectedIndex)
                                onClicked: {
                                    if (editorSplitView.mobileLayout) {
                                        window.selectUtterance(card.index);
                                        utteranceSettingsDialog.open();
                                    } else {
                                        cardMenu.open();
                                    }
                                }

                                Menu {
                                    id: cardMenu
                                    y: parent.height
                                    MenuItem {
                                        text: window.translator.tr("main.card.moveUp")
                                        enabled: card.index > 0
                                        onTriggered: {
                                            window.selectUtterance(card.index);
                                            window.moveUtterance(-1);
                                        }
                                    }
                                    MenuItem {
                                        text: window.translator.tr("main.card.moveDown")
                                        enabled: card.index < window.utterancesModel.count - 1
                                        onTriggered: {
                                            window.selectUtterance(card.index);
                                            window.moveUtterance(1);
                                        }
                                    }
                                    MenuSeparator {}
                                    MenuItem {
                                        text: window.translator.tr("main.card.delete")
                                        enabled: true
                                        onTriggered: {
                                            window.selectUtterance(card.index);
                                            window.removeUtterance();
                                        }
                                    }
                                }
                            }
                        }

                        Rectangle {
                            id: dragProxy
                            width: 42
                            height: 42
                            radius: 2
                            visible: imageDrag.active
                            color: window.palette.alternateBase
                            border.color: window.accent
                            opacity: 0.8
                            z: 20
                            Drag.active: imageDrag.active
                            Drag.source: card
                            Drag.keys: ["utterance"]
                            Drag.hotSpot.x: width / 2
                            Drag.hotSpot.y: height / 2

                            Image {
                                anchors.fill: parent
                                anchors.margins: 2
                                source: window.localImageUrl(card.imagePath)
                                fillMode: Image.PreserveAspectFit
                            }
                        }

                        DropArea {
                            anchors.fill: parent
                            keys: ["utterance"]
                            onEntered: drag => {
                                if (!drag.source)
                                    return;
                                const from = window.draggedUtteranceIndex;
                                const to = card.index;
                                if (from < 0 || to < 0 || from === to)
                                    return;
                                window.clearPlayback();
                                window.utterancesModel.move(from, to, 1);
                                window.selectedIndex = to;
                                window.draggedUtteranceIndex = to;
                                window.projectDirty = true;
                            }
                        }
                    }
                }

                RoundButton {
                    id: addButton
                    enabled: !window.batchExportActive
                    visible: !window.intonationLab
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.rightMargin: editorSplitView.mobileLayout ? 0 : 24
                    anchors.bottomMargin: editorSplitView.mobileLayout ? 0 : 8
                    width: editorSplitView.mobileLayout ? 52 : 48
                    height: width
                    highlighted: true
                    z: 2
                    contentItem: Text {
                        anchors.centerIn: parent
                        text: "\ue145"
                        color: addButton.palette.buttonText
                        font.family: iconFont.name
                        font.pixelSize: 28
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                    onClicked: window.addUtterance()
                    ToolTip.visible: hovered
                    ToolTip.text: window.translator.tr("main.addTooltip")
                }
            }

            Pane {
                id: parameterPane
                enabled: !window.batchExportActive
                parent: editorSplitView.mobileLayout ? utteranceSettingsDialog.contentItem : utteranceSplitView
                anchors.fill: editorSplitView.mobileLayout ? parent : undefined
                visible: !window.intonationLab && (!editorSplitView.mobileLayout || utteranceSettingsDialog.opened)
                SplitView.preferredWidth: visible ? 268 : 0
                SplitView.minimumWidth: visible ? 238 : 0
                SplitView.maximumWidth: visible ? 340 : 0
                padding: editorSplitView.mobileLayout ? 0 : 14
                background: Rectangle {
                    color: window.palette.window
                    border.color: window.borderColor
                    border.width: editorSplitView.mobileLayout ? 0 : 1
                }

                ScrollView {
                    id: parameterScroll
                    anchors.fill: parent
                    contentWidth: availableWidth
                    ScrollBar.vertical.policy: ScrollBar.AsNeeded

                    ColumnLayout {
                        width: Math.max(0, parameterScroll.availableWidth - (editorSplitView.mobileLayout ? 0 : 14))
                        spacing: 12

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 6

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 4
                            Label {
                                text: window.translator.tr("main.param.voicebank")
                                Layout.fillWidth: true
                            }
                            ComboBox {
                                id: voiceCombo
                                Layout.fillWidth: true
                                model: window.appBackend.voicebanks
                                textRole: "name"
                                valueRole: "id"
                                onActivated: {
                                window.updateSetting("voicebankId", currentValue);
                                const voice = window.voicebankById(currentValue);
                                window.utterancesModel.setProperty(window.selectedIndex, "imagePath", voice ? voice.image_path : "");
                                if (voice && voice.suggested_language) {
                                    const language = String(voice.suggested_language);
                                    const phonemizer = "auto";
                                    window.updateSpeechLanguage(language, phonemizer);
                                    window.selectCombo(speechLanguageCombo, language);
                                    window.selectCombo(phonemizerCombo, phonemizer);
                                }

                                const item = window.current();
                                if (!window.voicebankHasColor(currentValue, item.color || "")) {
                                    const options = window.voicebankTypeOptions(currentValue);
                                    window.updateSetting("color", options.length ? options[0].color : "");
                                }
                                Qt.callLater(() => window.selectCombo(colorCombo,
                                        window.typeIdForColor(currentValue, window.current().color || "")));
                                }
                            }
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 4
                            Label {
                                text: window.translator.tr("main.param.language")
                                Layout.fillWidth: true
                            }
                            ComboBox {
                                id: speechLanguageCombo
                                Layout.fillWidth: true
                                model: [
                                    { id: "ja", display_name: window.translator.tr("main.language.ja") },
                                    { id: "en", display_name: window.translator.tr("main.language.en") },
                                    { id: "zh", display_name: window.translator.tr("main.language.zh") }
                                ]
                                textRole: "display_name"
                                valueRole: "id"
                                onActivated: {
                                    const selectedPhonemizer = "auto";
                                    window.updateSpeechLanguage(currentValue, selectedPhonemizer);
                                    window.selectCombo(phonemizerCombo, selectedPhonemizer);
                                }
                            }
                        }

                        Label {
                            Layout.fillWidth: true
                            visible: window.appBackend.error.length > 0 || window.playbackError.length > 0
                            text: window.appBackend.error.length > 0 ? window.appBackend.error : window.playbackError
                            color: window.palette.text
                            wrapMode: Text.Wrap
                            font.pixelSize: 11
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 4
                            Label {
                                text: window.translator.tr("main.param.voicebankType")
                                Layout.fillWidth: true
                            }
                            ComboBox {
                                id: colorCombo
                                Layout.fillWidth: true
                                model: window.voicebankTypeOptions(
                                            window.utterancesModel.count ? window.current().voicebankId : "",
                                            window.utterancesModel.count ? window.current().color || "" : "")
                                textRole: "display_name"
                                valueRole: "id"
                                onActivated: {
                                    const type = window.voicebankTypeOptionAt(window.current().voicebankId,
                                            currentIndex, window.current().color || "");
                                    window.updateSetting("color", type ? type.color : "");
                                }
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                text: window.translator.tr("main.param.intonation")
                                Layout.fillWidth: true
                            }
                            SpinBox {
                                id: intonationInput
                                Layout.preferredWidth: 96
                                from: 0
                                to: Math.round(window.maxIntonationStrength * 100)
                                stepSize: 5
                                editable: true
                                value: Math.round(intonationSlider.value * 100)
                                textFromValue: value => (value / 100).toFixed(2)
                                valueFromText: text => Math.round(parseFloat(text) * 100)
                                onValueModified: {
                                    intonationSlider.value = value / 100;
                                    window.updateSetting("intonation", value / 100);
                                }
                            }
                        }
                        ParameterSlider {
                            id: intonationSlider
                            Layout.fillWidth: true
                            from: 0
                            to: window.maxIntonationStrength
                            stepSize: .05
                            onMoved: window.updateSetting("intonation", value)
                            onResetRequested: window.resetIntonation()
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                text: window.translator.tr("main.param.moraDuration")
                                Layout.fillWidth: true
                            }
                            SpinBox {
                                id: moraInput
                                Layout.preferredWidth: 96
                                from: 60
                                to: 300
                                stepSize: 5
                                editable: true
                                value: Math.round(moraSlider.value)
                                textFromValue: value => value + " ms"
                                valueFromText: text => parseInt(text)
                                onValueModified: {
                                    moraSlider.value = value;
                                    window.updateSetting("moraDuration", value);
                                }
                            }
                        }
                        ParameterSlider {
                            id: moraSlider
                            Layout.fillWidth: true
                            from: 60
                            to: 300
                            stepSize: 5
                            onMoved: window.updateSetting("moraDuration", value)
                            onResetRequested: window.resetMoraDuration()
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                text: window.translator.tr("main.param.pauseDuration")
                                Layout.fillWidth: true
                            }
                            SpinBox {
                                id: pauseInput
                                Layout.preferredWidth: 96
                                from: 0
                                to: 800
                                stepSize: 10
                                editable: true
                                value: Math.round(pauseSlider.value)
                                textFromValue: value => value + " ms"
                                valueFromText: text => parseInt(text)
                                onValueModified: {
                                    pauseSlider.value = value;
                                    window.updateSetting("pauseDuration", value);
                                }
                            }
                        }
                        ParameterSlider {
                            id: pauseSlider
                            Layout.fillWidth: true
                            from: 0
                            to: 800
                            stepSize: 10
                            onMoved: window.updateSetting("pauseDuration", value)
                            onResetRequested: window.resetPauseDuration()
                        }

                        Item {
                            Layout.fillWidth: true
                            Layout.preferredHeight: 24
                        }
                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 6

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 4
                            Label {
                                text: window.translator.tr("main.param.phonemizer")
                                Layout.fillWidth: true
                            }
                            ComboBox {
                                id: phonemizerCombo
                                Layout.fillWidth: true
                                model: window.phonemizerOptions(window.utterancesModel.count
                                        ? window.current().language || "ja" : "ja")
                                textRole: "display_name"
                                valueRole: "id"
                                onActivated: window.updateSetting("phonemizer", currentValue)
                            }
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 4
                            Label {
                                text: window.translator.tr("main.param.aliasPolicy")
                                Layout.fillWidth: true
                            }
                            ComboBox {
                                id: aliasPolicyCombo
                                Layout.fillWidth: true
                                model: [
                                    { id: "auto", display_name: window.translator.tr("main.aliasPolicy.auto") },
                                    { id: "cvvc-enhanced", display_name: window.translator.tr("main.aliasPolicy.cvvcEnhanced") },
                                    { id: "vcv-prefer", display_name: window.translator.tr("main.aliasPolicy.vcvPrefer") },
                                    { id: "cvvc-prefer", display_name: window.translator.tr("main.aliasPolicy.cvvcPrefer") },
                                    { id: "cv-only", display_name: window.translator.tr("main.aliasPolicy.cvOnly") }
                                ]
                                textRole: "display_name"
                                valueRole: "id"
                                onActivated: window.updateSetting("aliasPolicy", currentValue)
                            }
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 4
                            visible: !window.utterancesModel.count || ["ja", "en"].indexOf(window.current().language || "ja") >= 0
                            Label {
                                text: window.translator.tr("main.param.intonationModel")
                                Layout.fillWidth: true
                            }
                            ComboBox {
                                id: modelCombo
                                Layout.fillWidth: true
                                model: [{
                                    id: "none",
                                    display_name: window.translator.tr("main.modelNone")
                                }].concat(window.appBackend.models)
                                textRole: "display_name"
                                valueRole: "id"
                                onActivated: {
                                    window.updateSetting("modelId", currentValue);
                                    const model = window.modelById(currentValue);
                                    const renderer = window.preferredRendererForModel(model);
                                    if (renderer) {
                                        window.updateSetting("renderer", renderer);
                                        window.selectCombo(rendererCombo, renderer);
                                    }
                                }
                            }
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 4
                            Label {
                                text: window.translator.tr("main.param.renderer")
                                Layout.fillWidth: true
                            }
                            ComboBox {
                                id: rendererCombo
                                Layout.fillWidth: true
                                model: window.appBackend.renderers
                                textRole: "display_name"
                                valueRole: "id"
                                onActivated: window.updateSetting("renderer", currentValue)
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Layout.topMargin: 8
                            Label {
                                text: window.translator.tr("main.param.tone")
                                Layout.fillWidth: true
                            }
                            TextField {
                                id: toneField
                                Layout.preferredWidth: 72
                                horizontalAlignment: TextInput.AlignRight
                                text: "C4"
                                onEditingFinished: window.updateSetting("tone", text)
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Layout.topMargin: 8
                            Label {
                                text: window.translator.tr("main.param.leadingPreutterance")
                                Layout.fillWidth: true
                            }
                            SpinBox {
                                id: leadingPreutteranceInput
                                Layout.preferredWidth: 96
                                from: 0
                                to: 300
                                stepSize: 5
                                editable: true
                                property string automaticText: window.translator.tr("main.aliasPolicy.auto")
                                value: Math.round(leadingPreutteranceSlider.value)
                                textFromValue: value => value === 0
                                        ? leadingPreutteranceInput.automaticText : value + " ms"
                                Binding {
                                    target: leadingPreutteranceInput.contentItem
                                    property: "text"
                                    value: leadingPreutteranceInput.textFromValue(
                                               leadingPreutteranceInput.value,
                                               leadingPreutteranceInput.locale)
                                }
                                valueFromText: text => {
                                    const parsed = parseInt(text);
                                    return isNaN(parsed) ? 0 : parsed;
                                }
                                onValueModified: {
                                    leadingPreutteranceSlider.value = value;
                                    window.updateSetting("leadingPreutterance", value);
                                }
                            }
                        }
                        ParameterSlider {
                            id: leadingPreutteranceSlider
                            Layout.fillWidth: true
                            from: 0
                            to: 300
                            stepSize: 5
                            onMoved: window.updateSetting("leadingPreutterance", value)
                            onResetRequested: window.resetLeadingPreutterance()
                        }

                        }

                        Item {
                            Layout.fillHeight: true
                        }
                    }
                    }
                }
            }
        }

        Pane {
            id: pitchPane
            SplitView.fillHeight: window.intonationLab
            SplitView.preferredHeight: window.intonationLab ? 0 : editorSplitView.mobileLayout
                    ? Math.min(352, Math.max(150, editorSplitView.height - 96)) : 330
            SplitView.minimumHeight: 150
            padding: 0
            clip: true
            background: Rectangle {
                color: window.palette.window
            }

            ButtonGroup {
                id: pitchTabMode
                exclusive: true
            }
            ButtonGroup {
                id: pitchToolMode
                exclusive: true
            }

            ColumnLayout {
                anchors.fill: parent
                spacing: 0

                Item {
                    id: pitchEditorStage
                    Layout.fillWidth: true
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    Layout.fillHeight: true
                    Layout.bottomMargin: 8

                    Rectangle {
                        id: pitchEditorHeader
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.top: parent.top
                        anchors.leftMargin: 1
                        anchors.rightMargin: 1
                        anchors.topMargin: 1
                        height: 35
                        radius: 3
                        color: editorSplitView.mobileLayout ? "transparent" : window.palette.alternateBase
                        z: 1

                        Rectangle {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.bottom: parent.bottom
                            height: 4
                            color: parent.color
                        }
                        Rectangle {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.bottom: parent.bottom
                            height: 1
                            color: window.borderColor
                        }
                        Rectangle {
                            id: activePitchHeaderFill
                            property var activeTab: pitchModeTabs.currentIndex === 1
                                                    ? extendedPitchTab : basicPitchTab
                            x: activeTab ? activeTab.mapToItem(pitchEditorHeader, 0, 0).x : 0
                            width: activeTab ? activeTab.width : 0
                            height: parent.height
                            color: window.palette.base
                        }
                    }

                    Item {
                        id: pitchModeTabs
                        anchors.left: parent.left
                        anchors.top: parent.top
                        anchors.leftMargin: 1
                        anchors.topMargin: 1
                        width: basicPitchTab.width + extendedPitchTab.width
                        height: pitchEditorHeader.height
                        z: 3
                        readonly property int currentIndex: extendedPitchTab.checked ? 1 : 0

                        Row {
                            anchors.fill: parent
                            spacing: 0

                        ToolButton {
                            id: basicPitchTab
                            width: Math.max(96, basicPitchLabel.implicitWidth + 24)
                            height: parent.height
                            ButtonGroup.group: pitchTabMode
                            checkable: true
                            checked: true
                            text: window.translator.tr("main.pitch.basic")

                            background: Rectangle {
                                color: basicPitchTab.checked
                                       ? window.palette.base
                                       : basicPitchTab.hovered
                                         ? Qt.rgba(window.palette.alternateBase.r,
                                                   window.palette.alternateBase.g,
                                                   window.palette.alternateBase.b, 0.42)
                                         : "transparent"
                                Rectangle {
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.bottom: parent.bottom
                                    height: 1
                                    color: basicPitchTab.checked ? window.palette.base : "transparent"
                                }
                                Rectangle {
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 1
                                    height: 18
                                    color: window.borderColor
                                }
                            }
                            contentItem: Text {
                                id: basicPitchLabel
                                anchors.centerIn: parent
                                text: basicPitchTab.text
                                color: basicPitchTab.checked
                                       ? window.palette.text : window.palette.placeholderText
                                font.pixelSize: 13
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                            }
                        }
                        ToolButton {
                            id: extendedPitchTab
                            width: Math.max(96, extendedPitchLabel.implicitWidth + 24)
                            height: parent.height
                            ButtonGroup.group: pitchTabMode
                            checkable: true
                            text: window.translator.tr("main.pitch.extended")
                            onCheckedChanged: {
                                if (checked)
                                    window.scheduleExtendedEditorWaveform();
                            }

                            background: Rectangle {
                                color: extendedPitchTab.checked
                                       ? window.palette.base
                                       : extendedPitchTab.hovered
                                         ? Qt.rgba(window.palette.alternateBase.r,
                                                   window.palette.alternateBase.g,
                                                   window.palette.alternateBase.b, 0.42)
                                         : "transparent"
                                Rectangle {
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.bottom: parent.bottom
                                    height: 1
                                    color: extendedPitchTab.checked ? window.palette.base : "transparent"
                                }
                            }
                            contentItem: Text {
                                id: extendedPitchLabel
                                anchors.centerIn: parent
                                text: extendedPitchTab.text
                                color: extendedPitchTab.checked
                                       ? window.palette.text : window.palette.placeholderText
                                font.pixelSize: 13
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                            }
                        }
                        }
                    }
                    Rectangle {
                        id: pitchToolGroupFrame
                        visible: pitchModeTabs.currentIndex === 1
                        anchors.left: pitchModeTabs.right
                        anchors.top: parent.top
                        anchors.leftMargin: 12
                        anchors.topMargin: 1
                        width: visible ? 65 : 0
                        height: 35
                        z: 3
                        color: "transparent"

                        RowLayout {
                            anchors.fill: parent
                            spacing: 0

                            ToolButton {
                                id: handTool
                                ButtonGroup.group: pitchToolMode
                                checkable: true
                                checked: true
                                Layout.preferredWidth: 32
                                Layout.fillHeight: true
                                ToolTip.visible: hovered
                                ToolTip.text: window.translator.tr("tool.hand")
                                background: Rectangle {
                                    radius: 3
                                    color: handTool.checked
                                           ? Qt.rgba(window.accent.r, window.accent.g,
                                                     window.accent.b, 0.16)
                                           : handTool.hovered
                                             ? Qt.rgba(window.palette.mid.r, window.palette.mid.g,
                                                       window.palette.mid.b, 0.18)
                                             : "transparent"
                                }

                                contentItem: Text {
                                    anchors.centerIn: parent
                                    text: "\ue925"
                                    color: handTool.checked ? window.accent : handTool.palette.buttonText
                                    font.family: iconFont.name
                                    font.pixelSize: 16
                                    horizontalAlignment: Text.AlignHCenter
                                    verticalAlignment: Text.AlignVCenter
                                }
                            }
                            Rectangle {
                                Layout.preferredWidth: 1
                                Layout.preferredHeight: 14
                                Layout.alignment: Qt.AlignVCenter
                                color: window.borderColor
                            }
                            ToolButton {
                                id: penTool
                                ButtonGroup.group: pitchToolMode
                                checkable: true
                                Layout.preferredWidth: 32
                                Layout.fillHeight: true
                                ToolTip.visible: hovered
                                ToolTip.text: window.translator.tr("tool.pen")
                                background: Rectangle {
                                    radius: 3
                                    color: penTool.checked
                                           ? Qt.rgba(window.accent.r, window.accent.g,
                                                     window.accent.b, 0.16)
                                           : penTool.hovered
                                             ? Qt.rgba(window.palette.mid.r, window.palette.mid.g,
                                                       window.palette.mid.b, 0.18)
                                             : "transparent"
                                }

                                contentItem: Text {
                                    anchors.centerIn: parent
                                    text: "\ue3c9"
                                    color: penTool.checked ? window.accent : penTool.palette.buttonText
                                    font.family: iconFont.name
                                    font.pixelSize: 16
                                    horizontalAlignment: Text.AlignHCenter
                                    verticalAlignment: Text.AlignVCenter
                                }
                            }
                        }
                    }
                    IntonationEditorSurface {
                        id: pitchEditorSurface
                        anchors.fill: parent
                        surfaceColor: window.palette.base
                        borderColor: window.borderColor
                        contentMargin: 8
                        topContentMargin: 44
                        showSideBorders: false

                        StackLayout {
                            id: pitchModeStack
                            anchors.fill: parent
                            currentIndex: pitchModeTabs.currentIndex

                            ColumnLayout {
                                spacing: 0
                            PitchEditor {
                                id: pitchEditor
                                Layout.fillWidth: true
                                Layout.fillHeight: true
                                translator: window.translator
                                accentColor: window.accent
                                axisColor: window.palette.mid
                                gridColor: window.palette.alternateBase
                                labelColor: window.palette.text
                                defaultMoraDuration: window.appBackend.defaultMoraDuration
                                defaultPauseDuration: window.appBackend.defaultPauseDuration
                                onPointsEdited: points => window.updatePitchPoints(points)
                                onTimingEdited: (durations, positions) =>
                                        window.updateMoraTiming(durations, positions)
                            }
                            Item {
                                id: basicPitchScrollFooter
                                visible: basicPitchScrollBar.visible
                                Layout.fillWidth: true
                                Layout.preferredHeight: visible ? 18 : 0
                                Rectangle {
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.top: parent.top
                                    height: 1
                                    color: window.borderColor
                                }
                                PitchHorizontalScrollBar {
                                    id: basicPitchScrollBar
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.bottom: parent.bottom
                                    anchors.bottomMargin: 2
                                    editor: pitchEditor
                                    trackColor: window.palette.mid
                                    thumbColor: window.accent
                                }
                            }
                        }

                            PhonemeEditor {
                            id: phonemeEditor
                            translator: window.translator
                            accentColor: window.accent
                            axisColor: window.palette.mid
                            gridColor: window.palette.alternateBase
                            labelColor: window.palette.text
                            mutedText: window.mutedText
                            dividerColor: window.borderColor
                            showTimelineFrame: false
                            timingEditor: pitchEditor
                            units: window.extendedEditorUnits(pitchEditor.morae,
                                                              pitchEditor.moraDurations,
                                                              pitchEditor.moraPositions,
                                                              pitchEditor.defaultMoraDuration,
                                                              pitchEditor.defaultPauseDuration)
                            waveformMin: window.hasCurrentSynthesisView()
                                         ? window.synthesisWaveformMin : []
                            waveformMax: window.hasCurrentSynthesisView()
                                         ? window.synthesisWaveformMax : []
                            waveformDuration: window.hasCurrentSynthesisView()
                                              ? window.synthesisDurationMs : 0
                            leadingMargin: window.hasCurrentSynthesisLayout()
                                           ? window.synthesisLeadingMarginMs : 0
                            morae: pitchEditor.morae
                            moraDurations: pitchEditor.moraDurations
                            moraPositions: pitchEditor.moraPositions
                            overrides: window.utterancesModel.count
                                       ? window.decodeSequence(window.current().phonemeOverridesJson) : []
                            playbackMs: window.hasCurrentAudio() ? window.playerMedia.position : -1
                            showDetails: window.appBackend.extendedDetailsVisible
                            framePaintMode: penTool.checked && pitchModeTabs.currentIndex === 1
                            onUnitValueEdited: (unitIndex, key, value) =>
                                    window.updateUnitOverride(unitIndex, key, value)
                            onMoraStartEdited: (position, startMs) =>
                                    window.updateMoraStart(position, startMs)
                            onMoraDurationEdited: (position, durationMs) =>
                                    window.updateMoraDuration(position, durationMs)
                            onNoteGestureEdited: (durations, positions, points) =>
                                    window.updateTimingAndPitch(durations, positions, points)
                            onResetUnitRequested: (unitIndex) =>
                                    window.clearUnitOverride(unitIndex)
                            onSeekRequested: positionMs =>
                                    window.seekPreview(positionMs)
                            onFramesEdited: frames => window.updatePitchFrames(frames)
                        }
                    }
                }
            }
                PlaybackControls {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 48
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    translator: window.translator
                    mutedText: window.mutedText
                    busy: window.appBackend.busy
                    playing: !window.batchExportActive
                             && window.playerMedia.playbackState === MediaPlayer.PlayingState
                    hasAudio: !window.batchExportActive && window.hasCurrentAudio()
                    canGenerate: !window.batchExportActive && window.utterancesModel.count
                                 && window.current().reading.length > 0
                                 && (!window.appBackend.busy
                                     || window.pendingUtteranceId === window.current().utteranceId
                                        && window.pendingRevision === window.current().revision)
                    position: window.playerMedia.position
                    duration: window.playerMedia.duration
                    errorText: window.appBackend.error.length ? window.appBackend.error : window.playbackError
                    onPrimaryClicked: {
                        if (window.playerMedia.playbackState === MediaPlayer.PlayingState)
                            window.playerMedia.pause();
                        else if (window.hasCurrentAudio()) {
                            if (window.playerMedia.duration > 0
                                    && window.playerMedia.position >= window.playerMedia.duration - 1)
                                window.playerMedia.position = 0;
                            window.playerMedia.play();
                        } else {
                            window.synthesizeCurrent();
                        }
                    }
                    onSeekRequested: position => window.playerMedia.position = position
                }
            }
        }

        MobilePageDialog {
            id: utteranceSettingsDialog
            title: window.translator.tr("menu.settings")
            contentItem: Item {}
            footer: Pane {
                visible: editorSplitView.mobileLayout
                leftPadding: utteranceSettingsDialog.pageMargin
                rightPadding: utteranceSettingsDialog.pageMargin
                topPadding: 0
                bottomPadding: utteranceSettingsDialog.pageMargin
                background: Item {}
                contentItem: Button {
                    text: window.translator.tr("main.card.delete")
                    onClicked: {
                        window.removeUtterance();
                        utteranceSettingsDialog.close();
                    }
                }
            }
        }

        onMobileLayoutChanged: {
            if (!mobileLayout)
                utteranceSettingsDialog.close();
        }
    }
