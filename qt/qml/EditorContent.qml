pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtMultimedia
    SplitView {
        required property var window

        property alias pitchEditor: pitchEditor
        property alias utteranceList: utteranceList
        property alias voiceCombo: voiceCombo
        property alias speechLanguageCombo: speechLanguageCombo
        property alias phonemizerCombo: phonemizerCombo
        property alias aliasPolicyCombo: aliasPolicyCombo
        property alias modelCombo: modelCombo
        property alias rendererCombo: rendererCombo
        property alias resamplerCombo: resamplerCombo
        property alias wavtoolCombo: wavtoolCombo
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

        anchors.fill: parent
        orientation: Qt.Vertical

        SplitView {
            SplitView.fillHeight: true
            orientation: Qt.Horizontal

            Pane {
                SplitView.fillWidth: true
                SplitView.minimumWidth: 560
                padding: 10
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
                    bottomMargin: 64
                    ScrollBar.vertical: ScrollBar {
                        id: utteranceScrollBar
                        policy: ScrollBar.AlwaysOn
                    }

                    delegate: Item {
                        id: card
                        required property int index
                        required property string content
                        required property string voicebankId
                        required property string imagePath
                        property alias textEditor: utteranceEditor

                        width: Math.max(0, utteranceList.width - 14 - 2)
                        height: 46

                        RowLayout {
                            anchors.fill: parent
                            spacing: 6

                            Rectangle {
                                id: imageHandle
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
                                            && !window.settingsWindowRef.visible
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
                                contentItem: BreezeIcon {
                                    anchors.centerIn: parent
                                    width: 22
                                    height: 22
                                    source: "qrc:/icons/breeze-overflow-menu.svg"
                                    iconColor: cardMenuButton.palette.buttonText
                                }
                                visible: card.index === window.selectedIndex
                                onClicked: cardMenu.open()

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
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.rightMargin: 24
                    anchors.bottomMargin: 8
                    width: 48
                    height: 48
                    highlighted: true
                    z: 2
                    contentItem: Canvas {
                        anchors.fill: parent
                        property color iconColor: addButton.palette.buttonText
                        onIconColorChanged: requestPaint()
                        onPaint: {
                            const context = getContext("2d");
                            context.clearRect(0, 0, width, height);
                            context.strokeStyle = iconColor;
                            context.lineWidth = 2.4;
                            context.lineCap = "round";
                            context.beginPath();
                            context.moveTo(width * 0.3, height * 0.5);
                            context.lineTo(width * 0.7, height * 0.5);
                            context.moveTo(width * 0.5, height * 0.3);
                            context.lineTo(width * 0.5, height * 0.7);
                            context.stroke();
                        }
                    }
                    onClicked: window.addUtterance()
                    ToolTip.visible: hovered
                    ToolTip.text: window.translator.tr("main.addTooltip")
                }
            }

            Pane {
                SplitView.preferredWidth: 268
                SplitView.minimumWidth: 238
                SplitView.maximumWidth: 340
                padding: 14
                background: Rectangle {
                    color: window.palette.window
                    border.color: window.borderColor
                }

                ScrollView {
                    id: parameterScroll
                    anchors.fill: parent
                    contentWidth: availableWidth
                    ScrollBar.vertical.policy: ScrollBar.AsNeeded

                    ColumnLayout {
                        width: Math.max(0, parameterScroll.availableWidth - 14)
                        spacing: 12

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 4

                        Label {
                            text: window.translator.tr("main.param.voicebank")
                            font.pixelSize: 12
                            color: window.mutedText
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
                                if (window.appBackend.developerMode && voice && voice.suggested_language) {
                                    const language = String(voice.suggested_language);
                                    const phonemizer = String(voice.suggested_phonemizer
                                                              || window.defaultPhonemizer(language));
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

                        Label {
                            Layout.topMargin: 8
                            visible: window.appBackend.developerMode
                            text: window.translator.tr("main.param.language")
                            font.pixelSize: 12
                            color: window.mutedText
                        }
                        ComboBox {
                            id: speechLanguageCombo
                            visible: window.appBackend.developerMode
                            Layout.fillWidth: true
                            model: [
                                { id: "ja", display_name: window.translator.tr("main.language.ja") },
                                { id: "en", display_name: window.translator.tr("main.language.en") },
                                { id: "zh", display_name: window.translator.tr("main.language.zh") }
                            ]
                            textRole: "display_name"
                            valueRole: "id"
                            onActivated: {
                                const selectedPhonemizer = window.defaultPhonemizer(currentValue);
                                window.updateSpeechLanguage(currentValue, selectedPhonemizer);
                                window.selectCombo(phonemizerCombo, selectedPhonemizer);
                            }
                        }

                        Label {
                            Layout.topMargin: 8
                            visible: window.appBackend.developerMode
                            text: window.translator.tr("main.param.phonemizer")
                            font.pixelSize: 12
                            color: window.mutedText
                        }
                        ComboBox {
                            id: phonemizerCombo
                            visible: window.appBackend.developerMode
                            Layout.fillWidth: true
                            model: window.phonemizerOptions(window.utterancesModel.count
                                    ? window.current().language || "ja" : "ja")
                            textRole: "display_name"
                            valueRole: "id"
                            onActivated: window.updateSetting("phonemizer", currentValue)
                        }

                        Label {
                            Layout.topMargin: 8
                            text: window.translator.tr("main.param.aliasPolicy")
                            color: window.mutedText
                            font.pixelSize: 12
                        }
                        ComboBox {
                            id: aliasPolicyCombo
                            Layout.fillWidth: true
                            model: [
                                { id: "auto", display_name: window.translator.tr("main.aliasPolicy.auto") },
                                { id: "legacy", display_name: window.translator.tr("main.aliasPolicy.legacy") },
                                { id: "cvvc-enhanced", display_name: window.translator.tr("main.aliasPolicy.cvvcEnhanced") },
                                { id: "vcv-prefer", display_name: window.translator.tr("main.aliasPolicy.vcvPrefer") },
                                { id: "cvvc-prefer", display_name: window.translator.tr("main.aliasPolicy.cvvcPrefer") },
                                { id: "cv-only", display_name: window.translator.tr("main.aliasPolicy.cvOnly") }
                            ]
                            textRole: "display_name"
                            valueRole: "id"
                            onActivated: window.updateSetting("aliasPolicy", currentValue)
                        }

                        Label {
                            Layout.topMargin: 8
                            text: window.translator.tr("main.param.intonationModel")
                            font.pixelSize: 12
                            color: window.mutedText
                        }
                        ComboBox {
                            id: modelCombo
                            Layout.fillWidth: true
                            model: [
                                {
                                    id: "none",
                                    display_name: window.translator.tr("main.modelNone")
                                }
                            ].concat(window.appBackend.models)
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

                        Label {
                            Layout.topMargin: 8
                            text: window.translator.tr("main.param.renderer")
                            font.pixelSize: 12
                            color: window.mutedText
                        }
                        ComboBox {
                            id: rendererCombo
                            Layout.fillWidth: true
                            model: window.appBackend.renderers
                            textRole: "display_name"
                            valueRole: "id"
                            onActivated: window.updateSetting("renderer", currentValue)
                        }
                        Label {
                            Layout.topMargin: 8
                            visible: rendererCombo.currentValue === "classic-utau"
                            text: window.translator.tr("main.param.resampler")
                            font.pixelSize: 12
                            color: window.mutedText
                        }
                        ComboBox {
                            id: resamplerCombo
                            visible: rendererCombo.currentValue === "classic-utau"
                            Layout.fillWidth: true
                            model: window.appBackend.resamplers
                            textRole: "display_name"
                            valueRole: "id"
                            onActivated: window.updateSetting("resampler", currentValue)
                        }
                        Label {
                            Layout.topMargin: 8
                            visible: rendererCombo.currentValue === "classic-utau"
                            text: window.translator.tr("main.param.wavtool")
                            font.pixelSize: 12
                            color: window.mutedText
                        }
                        ComboBox {
                            id: wavtoolCombo
                            visible: rendererCombo.currentValue === "classic-utau"
                            Layout.fillWidth: true
                            model: window.appBackend.wavtools
                            textRole: "display_name"
                            valueRole: "id"
                            onActivated: window.updateSetting("wavtool", currentValue)
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

                        RowLayout {
                            Layout.fillWidth: true
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
                                Layout.preferredWidth: 86
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
                        Item {
                            Layout.fillWidth: true
                            Layout.preferredHeight: intonationSlider.implicitHeight
                            Slider {
                                id: intonationSlider
                                anchors.fill: parent
                                from: 0
                                to: window.maxIntonationStrength
                                stepSize: .05
                                onMoved: {
                                    intonationInput.value = Math.round(value * 100);
                                    window.updateSetting("intonation", value);
                                }
                            }
                            MouseArea {
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                onPressed: mouse => updateAt(mouse.x)
                                onPositionChanged: mouse => {
                                    if (pressed)
                                        updateAt(mouse.x);
                                }
                                onDoubleClicked: window.resetIntonation()
                                function updateAt(x) {
                                    const fraction = Math.max(0, Math.min(1, x / width));
                                    const value = Math.round((intonationSlider.from + fraction * (intonationSlider.to - intonationSlider.from)) / intonationSlider.stepSize) * intonationSlider.stepSize;
                                    intonationSlider.value = value;
                                    intonationInput.value = Math.round(value * 100);
                                    window.updateSetting("intonation", value);
                                }
                            }
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
                        Item {
                            Layout.fillWidth: true
                            Layout.preferredHeight: moraSlider.implicitHeight
                            Slider {
                                id: moraSlider
                                anchors.fill: parent
                                from: 60
                                to: 300
                                stepSize: 5
                                onMoved: {
                                    window.updateSetting("moraDuration", value);
                                    moraInput.value = value;
                                }
                            }
                            MouseArea {
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                onPressed: mouse => updateAt(mouse.x)
                                onPositionChanged: mouse => {
                                    if (pressed)
                                        updateAt(mouse.x);
                                }
                                onDoubleClicked: window.resetMoraDuration()
                                function updateAt(x) {
                                    const fraction = Math.max(0, Math.min(1, x / width));
                                    const value = Math.round((moraSlider.from + fraction * (moraSlider.to - moraSlider.from)) / moraSlider.stepSize) * moraSlider.stepSize;
                                    moraSlider.value = value;
                                    moraInput.value = value;
                                    window.updateSetting("moraDuration", value);
                                }
                            }
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
                        Item {
                            Layout.fillWidth: true
                            Layout.preferredHeight: pauseSlider.implicitHeight
                            Slider {
                                id: pauseSlider
                                anchors.fill: parent
                                from: 0
                                to: 800
                                stepSize: 10
                                onMoved: {
                                    window.updateSetting("pauseDuration", value);
                                    pauseInput.value = value;
                                }
                            }
                            MouseArea {
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                onPressed: mouse => updateAt(mouse.x)
                                onPositionChanged: mouse => {
                                    if (pressed)
                                        updateAt(mouse.x);
                                }
                                onDoubleClicked: window.resetPauseDuration()
                                function updateAt(x) {
                                    const fraction = Math.max(0, Math.min(1, x / width));
                                    const value = Math.round((pauseSlider.from + fraction * (pauseSlider.to - pauseSlider.from)) / pauseSlider.stepSize) * pauseSlider.stepSize;
                                    pauseSlider.value = value;
                                    pauseInput.value = value;
                                    window.updateSetting("pauseDuration", value);
                                }
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
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
                                value: Math.round(leadingPreutteranceSlider.value)
                                textFromValue: value => value + " ms"
                                Component.onCompleted: refreshTextFormatter()
                                function refreshTextFormatter() {
                                    const automaticText = window.translator.tr("main.aliasPolicy.auto");
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
                                onValueModified: {
                                    leadingPreutteranceSlider.value = value;
                                    window.updateSetting("leadingPreutterance", value);
                                }
                                Connections {
                                    target: window.translator
                                    function onTranslationsChanged() {
                                        leadingPreutteranceInput.refreshTextFormatter();
                                    }
                                }
                            }
                        }
                        Item {
                            Layout.fillWidth: true
                            Layout.preferredHeight: leadingPreutteranceSlider.implicitHeight
                            Slider {
                                id: leadingPreutteranceSlider
                                anchors.fill: parent
                                from: 0
                                to: 300
                                stepSize: 5
                                onMoved: {
                                    window.updateSetting("leadingPreutterance", value);
                                    leadingPreutteranceInput.value = value;
                                }
                            }
                            MouseArea {
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                onPressed: mouse => updateAt(mouse.x)
                                onPositionChanged: mouse => {
                                    if (pressed)
                                        updateAt(mouse.x);
                                }
                                onDoubleClicked: window.resetLeadingPreutterance()
                                function updateAt(x) {
                                    const fraction = Math.max(0, Math.min(1, x / width));
                                    const value = Math.round((leadingPreutteranceSlider.from + fraction * (leadingPreutteranceSlider.to - leadingPreutteranceSlider.from)) / leadingPreutteranceSlider.stepSize) * leadingPreutteranceSlider.stepSize;
                                    leadingPreutteranceSlider.value = value;
                                    leadingPreutteranceInput.value = value;
                                    window.updateSetting("leadingPreutterance", value);
                                }
                            }
                        }

                        Item {
                            Layout.fillHeight: true
                        }
                    }
                }
            }
        }

        Pane {
            SplitView.preferredHeight: 238
            SplitView.minimumHeight: 150
            padding: 0
            background: Rectangle {
                color: window.palette.window
                border.color: window.borderColor
            }

            ColumnLayout {
                anchors.fill: parent
                spacing: 0

                RowLayout {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 38
                    Layout.leftMargin: 12
                    Layout.rightMargin: 10
                    Label {
                        text: window.translator.tr("main.pitch.title")
                        font.pixelSize: 12
                    }
                    Item {
                        Layout.fillWidth: true
                    }
                }
                Rectangle {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 1
                    color: window.borderColor
                }
                PitchEditor {
                    id: pitchEditor
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    translator: window.translator
                    accentColor: window.accent
                    axisColor: window.palette.mid
                    gridColor: window.palette.alternateBase
                    labelColor: window.palette.text
                    defaultMoraDuration: window.appBackend.defaultMoraDuration
                    defaultPauseDuration: window.appBackend.defaultPauseDuration
                    onPointsEdited: points => window.updatePitchPoints(points)
                    onMoraDurationsEdited: durations => window.updateMoraDurations(durations)
                    onMoraPositionsEdited: positions => window.updateMoraPositions(positions)
                }
                PitchHorizontalScrollBar {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 14
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    editor: pitchEditor
                    trackColor: window.palette.mid
                    thumbColor: window.accent
                }
                PlaybackControls {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 52
                    Layout.leftMargin: 10
                    Layout.rightMargin: 10
                    translator: window.translator
                    mutedText: window.mutedText
                    busy: window.appBackend.busy
                    playing: !window.batchExportActive
                             && window.playerMedia.playbackState === MediaPlayer.PlayingState
                    hasAudio: !window.batchExportActive && window.hasCurrentAudio()
                    canGenerate: !window.batchExportActive && window.utterancesModel.count
                                 && window.current().reading.length > 0
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
    }
