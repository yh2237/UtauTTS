pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import UtauTTS.Media
import UtauTTSStyle
    SplitView {
        id: editorSplitView
        required property var window
        readonly property bool mobileLayout: window.mobileLayout
        readonly property bool extendedPitchEditorVisible: pitchModeTabs.currentIndex === 1
        readonly property bool referenceMatching: methodCombo.currentValue === "reference-matching"
        // 合成後に音量を動かすと、次のプレビューまで原音エディタはモーラ単位の仮の並びになる。番号がずれないよう、同じ文章の合成結果を使う。
        readonly property var volumeUnits: window.hasCurrentSynthesisLayout() && window.synthesisUnits.length
                                           ? window.synthesisUnits : phonemeEditor.units

        property alias pitchEditor: pitchEditor
        property alias phonemeEditor: phonemeEditor
        property alias utteranceList: utteranceList
        property alias voiceCombo: voiceCombo
        property alias speechLanguageCombo: speechLanguageCombo
        property alias methodCombo: methodCombo
        property alias phonemizerCombo: phonemizerCombo
        property alias aliasPolicyCombo: aliasPolicyCombo
        property alias modelCombo: modelCombo
        property alias toneInput: toneInput
        property alias toneSlider: toneSlider
        property alias colorCombo: colorCombo
        property alias intonationInput: intonationInput
        property alias intonationSlider: intonationSlider
        property alias moraInput: moraInput
        property alias moraSlider: moraSlider
        property alias pauseInput: pauseInput
        property alias pauseSlider: pauseSlider
        property alias volumeSlider: volumeSlider
        property alias leadingPreutteranceInput: leadingPreutteranceInput
        property alias leadingPreutteranceSlider: leadingPreutteranceSlider

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
        // 隣に枠が無いので、左右の仕切りと同じく両脇に枠の線を引く。
        handle: Item {
            id: verticalHandle
            readonly property bool hovered: SplitHandle.hovered
            readonly property bool pressed: SplitHandle.pressed
            implicitHeight: verticalGrip.extent + 2

            Rectangle {
                width: parent.width
                height: 1
                color: Theme.frameLine
            }
            StyleItem {
                id: verticalGrip
                y: 1
                width: parent.width
                height: extent
                element: StyleItem.Splitter
                orientation: Qt.Vertical
                hovered: verticalHandle.hovered
                pressed: verticalHandle.pressed
            }
            Rectangle {
                y: parent.height - 1
                width: parent.width
                height: 1
                color: Theme.frameLine
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
                padding: 0
                background: Rectangle {
                    color: window.palette.window
                }

                ListView {
                    id: utteranceList
                    readonly property real scrollBarWidth: utteranceScrollBar.visible ? utteranceScrollBar.width : 0
                    anchors.fill: parent
                    model: window.utterancesModel
                    clip: true
                    spacing: Theme.spacingSmall
                    boundsBehavior: Flickable.StopAtBounds
                    leftMargin: Theme.spacingLarge
                    rightMargin: Theme.spacingLarge + scrollBarWidth
                    topMargin: Theme.spacingLarge
                    bottomMargin: window.intonationLab ? Theme.spacingLarge : addButton.height + Theme.spacingLarge * 2
                    ScrollBar.vertical: ScrollBar {
                        id: utteranceScrollBar
                        anchors.right: parent.right
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
                        width: Math.max(0, utteranceList.width - utteranceList.leftMargin - utteranceList.rightMargin)
                        height: visible ? cardRow.implicitHeight : 0

                        RowLayout {
                            id: cardRow
                            anchors.fill: parent
                            spacing: Theme.spacingMedium

                            Rectangle {
                                id: imageHandle
                                visible: !window.intonationLab
                                Layout.preferredWidth: Theme.cardHeight
                                Layout.preferredHeight: Theme.cardHeight
                                radius: Theme.radius
                                color: window.palette.alternateBase
                                border.color: card.index === window.selectedIndex ? Theme.selection : Theme.border

                                Image {
                                    anchors.fill: parent
                                    anchors.margins: Theme.spacingXSmall
                                    source: window.localImageUrl(card.imagePath)
                                    fillMode: Image.PreserveAspectFit
                                    asynchronous: true
                                }
                                Label {
                                    anchors.centerIn: parent
                                    visible: !card.imagePath
                                    text: window.translator.tr("main.card.icon")
                                    color: Theme.inactiveText
                                    font.pixelSize: Theme.fontXSmall
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
                                Layout.preferredHeight: Theme.cardHeight
                                text: card.content
                                font.pixelSize: Theme.fontLarge
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

                            IconButton {
                                id: cardMenuButton
                                enabled: !window.batchExportActive
                                glyph: editorSplitView.mobileLayout ? "\ue3c9" : "\ue5d4"
                                glyphSize: Theme.icon
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
                            width: imageHandle.width
                            height: imageHandle.height
                            radius: Theme.radius
                            visible: imageDrag.active
                            color: window.palette.alternateBase
                            border.color: Theme.selection
                            opacity: 0.8
                            z: 20
                            Drag.active: imageDrag.active
                            Drag.source: card
                            Drag.keys: ["utterance"]
                            Drag.hotSpot.x: width / 2
                            Drag.hotSpot.y: height / 2

                            Image {
                                anchors.fill: parent
                                anchors.margins: Theme.spacingXSmall
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

                AccentRoundButton {
                    id: addButton
                    enabled: !window.batchExportActive
                    visible: !window.intonationLab
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.rightMargin: Theme.spacingLarge + utteranceList.scrollBarWidth
                    anchors.bottomMargin: Theme.spacingLarge
                    z: 2
                    glyph: "\ue145"
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
                padding: editorSplitView.mobileLayout ? 0 : parameterFrame.paddingLeft
                background: StyleItem {
                    id: parameterFrame
                    visible: !editorSplitView.mobileLayout
                    element: StyleItem.Frame
                }

                ScrollView {
                    id: parameterScroll
                    anchors.fill: parent
                    contentWidth: availableWidth
                    contentHeight: parameterColumn.implicitHeight + Theme.spacingLarge * 2
                    ScrollBar.vertical.policy: ScrollBar.AsNeeded

                    ColumnLayout {
                        id: parameterColumn
                        x: Theme.spacingLarge
                        y: Theme.spacingLarge
                        width: Math.max(0, parameterScroll.availableWidth - Theme.spacingLarge * 2)
                        spacing: Theme.spacingLarge

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: Theme.spacingMedium

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: Theme.spacingSmall
                            Label {
                                text: window.translator.tr("main.param.voicebank")
                                Layout.fillWidth: true
                            }
                            ComboBox {
                                id: voiceCombo
                                objectName: "voicebankCombo"
                                Layout.fillWidth: true
                                model: window.appBackend.voicebanks
                                textRole: "name"
                                valueRole: "id"
                                onActivated: {
                                window.updateUtteranceField("voicebankId", currentValue);
                                const voice = window.voicebankById(currentValue);
                                window.utterancesModel.setProperty(window.selectedIndex, "imagePath", voice ? voice.image_path : "");
                                if (voice && voice.suggested_language) {
                                    const language = String(voice.suggested_language);
                                    const phonemizer = "auto";
                                    const method = window.defaultMethod(language);
                                    window.utterancesModel.setProperty(window.selectedIndex, "method", method);
                                    window.updateSpeechLanguage(language, phonemizer);
                                    Qt.callLater(function() {
                                        window.selectCombo(methodCombo, method);
                                        window.selectCombo(speechLanguageCombo, language);
                                        window.selectCombo(phonemizerCombo, phonemizer);
                                    });
                                }

                                const item = window.current();
                                if (!window.voicebankHasColor(currentValue, item.color || "")) {
                                    const options = window.voicebankTypeOptions(currentValue);
                                    window.updateUtteranceField("color", options.length ? options[0].color : "");
                                }
                                Qt.callLater(() => window.selectCombo(colorCombo,
                                        window.typeIdForColor(currentValue, window.current().color || "")));
                                }
                            }
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: Theme.spacingSmall
                            Label {
                                text: window.translator.tr("main.param.method")
                                Layout.fillWidth: true
                            }
                            ComboBox {
                                id: methodCombo
                                Layout.fillWidth: true
                                model: window.methodOptions()
                                textRole: "display_name"
                                valueRole: "id"
                                onActivated: window.updateSpeechMethod(currentValue)
                            }
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: Theme.spacingSmall
                            Label {
                                text: window.translator.tr("main.param.language")
                                Layout.fillWidth: true
                            }
                            ComboBox {
                                id: speechLanguageCombo
                                Layout.fillWidth: true
                                model: window.languageOptions(methodCombo.currentValue)
                                textRole: "display_name"
                                valueRole: "id"
                                onActivated: {
                                    const selectedPhonemizer = "auto";
                                    window.updateSpeechLanguage(currentValue, selectedPhonemizer);
                                    Qt.callLater(function() {
                                        window.selectCombo(phonemizerCombo, selectedPhonemizer);
                                    });
                                }
                            }
                        }

                        Label {
                            Layout.fillWidth: true
                            visible: window.appBackend.error.length > 0 || window.playbackError.length > 0
                            text: window.appBackend.error.length > 0 ? window.appBackend.error : window.playbackError
                            color: window.palette.text
                            wrapMode: Text.Wrap
                            font.pixelSize: Theme.fontSmall
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: Theme.spacingSmall
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
                                    window.updateUtteranceField("color", type ? type.color : "");
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
                                Layout.preferredWidth: Theme.fieldWidthSmall
                                from: 0
                                to: Math.round(window.maxIntonationStrength * 100)
                                stepSize: 5
                                editable: true
                                value: Math.round(intonationSlider.value * 100)
                                textFromValue: value => (value / 100).toFixed(2)
                                valueFromText: text => Math.round(parseFloat(text) * 100)
                                onValueModified: {
                                    intonationSlider.value = value / 100;
                                    window.updateUtteranceField("intonation", value / 100);
                                }
                            }
                        }
                        ParameterSlider {
                            id: intonationSlider
                            Layout.fillWidth: true
                            from: 0
                            to: window.maxIntonationStrength
                            stepSize: .05
                            onMoved: window.updateUtteranceField("intonation", value)
                            onResetRequested: window.resetIntonation()
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                text: window.translator.tr("main.param.tone")
                                Layout.fillWidth: true
                            }
                            ToneSpinBox {
                                id: toneInput
                                Layout.preferredWidth: Theme.fieldWidthSmall
                                automaticText: window.translator.tr("main.aliasPolicy.auto")
                                value: Math.round(toneSlider.value)
                                onToneModified: tone => {
                                    toneSlider.value = value;
                                    window.updateUtteranceField("tone", tone);
                                }
                            }
                        }
                        ParameterSlider {
                            id: toneSlider
                            Layout.fillWidth: true
                            from: toneInput.from
                            to: toneInput.to
                            onMoved: window.updateUtteranceField("tone", toneInput.valueToTone(Math.round(value)))
                            onResetRequested: window.resetTone()
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Label {
                                text: window.translator.tr("main.param.moraDuration")
                                Layout.fillWidth: true
                            }
                            SpinBox {
                                id: moraInput
                                Layout.preferredWidth: Theme.fieldWidthSmall
                                from: 60
                                to: 300
                                stepSize: 5
                                editable: true
                                value: Math.round(moraSlider.value)
                                textFromValue: value => value + " ms"
                                valueFromText: text => parseInt(text)
                                onValueModified: {
                                    moraSlider.value = value;
                                    window.updateUtteranceField("moraDuration", value);
                                }
                            }
                        }
                        ParameterSlider {
                            id: moraSlider
                            Layout.fillWidth: true
                            from: 60
                            to: 300
                            stepSize: 5
                            onMoved: window.updateUtteranceField("moraDuration", value)
                            onResetRequested: window.resetMoraDuration()
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            visible: !editorSplitView.referenceMatching
                            Label {
                                text: window.translator.tr("main.param.pauseDuration")
                                Layout.fillWidth: true
                            }
                            SpinBox {
                                id: pauseInput
                                Layout.preferredWidth: Theme.fieldWidthSmall
                                from: 0
                                to: 800
                                stepSize: 10
                                editable: true
                                value: Math.round(pauseSlider.value)
                                textFromValue: value => value + " ms"
                                valueFromText: text => parseInt(text)
                                onValueModified: {
                                    pauseSlider.value = value;
                                    window.updateUtteranceField("pauseDuration", value);
                                }
                            }
                        }
                        ParameterSlider {
                            id: pauseSlider
                            visible: !editorSplitView.referenceMatching
                            Layout.fillWidth: true
                            from: 0
                            to: 800
                            stepSize: 10
                            onMoved: window.updateUtteranceField("pauseDuration", value)
                            onResetRequested: window.resetPauseDuration()
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            enabled: editorSplitView.volumeUnits.length > 0
                            Label {
                                text: window.translator.tr("main.param.volume")
                                Layout.fillWidth: true
                            }
                            SpinBox {
                                id: volumeInput
                                Layout.preferredWidth: Theme.fieldWidthSmall
                                from: 0
                                to: 200
                                stepSize: 5
                                editable: true
                                value: Math.round(volumeSlider.value)
                                textFromValue: value => value + " %"
                                valueFromText: text => parseInt(text)
                                onValueModified: {
                                    volumeSlider.value = value;
                                    window.updateAllUnitVolume(editorSplitView.volumeUnits, value);
                                }
                            }
                        }
                        ParameterSlider {
                            id: volumeSlider
                            Layout.fillWidth: true
                            enabled: editorSplitView.volumeUnits.length > 0
                            from: 0
                            to: 200
                            stepSize: 5
                            onMoved: window.updateAllUnitVolume(editorSplitView.volumeUnits, value)
                            onResetRequested: window.resetAllUnitVolume()
                        }
                        Binding {
                            target: volumeSlider
                            property: "value"
                            value: window.commonUnitVolume(editorSplitView.volumeUnits, phonemeEditor.overrides)
                            when: !volumeSlider.pressed
                            restoreMode: Binding.RestoreNone
                        }

                        Item {
                            Layout.fillWidth: true
                            Layout.preferredHeight: Theme.spacingXLarge + Theme.spacing
                        }
                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: Theme.spacingMedium

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: Theme.spacingSmall
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
                                onActivated: window.updateUtteranceField("phonemizer", currentValue)
                            }
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            visible: !editorSplitView.referenceMatching
                            spacing: Theme.spacingSmall
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
                                onActivated: window.updateUtteranceField("aliasPolicy", currentValue)
                            }
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: Theme.spacingSmall
                            visible: !editorSplitView.referenceMatching && (!window.utterancesModel.count || ["ja", "en"].indexOf(window.current().language || "ja") >= 0)
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
                                onActivated: window.updateUtteranceField("modelId", currentValue)
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            visible: !editorSplitView.referenceMatching
                            Layout.topMargin: Theme.spacing
                            Label {
                                text: window.translator.tr("main.param.leadingPreutterance")
                                Layout.fillWidth: true
                            }
                            SpinBox {
                                id: leadingPreutteranceInput
                                Layout.preferredWidth: Theme.fieldWidthSmall
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
                                    window.updateUtteranceField("leadingPreutterance", value);
                                }
                            }
                        }
                        ParameterSlider {
                            id: leadingPreutteranceSlider
                            visible: !editorSplitView.referenceMatching
                            Layout.fillWidth: true
                            from: 0
                            to: 300
                            stepSize: 5
                            onMoved: window.updateUtteranceField("leadingPreutterance", value)
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
                id: pitchToolMode
                exclusive: true
            }

            ColumnLayout {
                anchors.fill: parent
                spacing: 0

                Item {
                    id: pitchEditorStage
                    Layout.fillWidth: true
                    Layout.leftMargin: Theme.spacingLarge
                    Layout.rightMargin: Theme.spacingLarge
                    Layout.fillHeight: true
                    Layout.topMargin: Theme.spacing
                    Layout.bottomMargin: Theme.spacing

                    TabBar {
                        id: pitchModeTabs
                        objectName: "pitchModeTabs"
                        anchors.left: parent.left
                        anchors.top: parent.top
                        font.pixelSize: Theme.fontMedium
                        z: 3

                        DocumentTabButton {
                            id: basicPitchTab
                            width: implicitWidth
                            text: window.translator.tr("main.pitch.basic")
                        }
                        DocumentTabButton {
                            id: extendedPitchTab
                            width: implicitWidth
                            text: window.translator.tr("main.pitch.extended")
                            onCheckedChanged: {
                                if (checked)
                                    window.scheduleExtendedEditorWaveform();
                            }
                        }
                    }
                    RowLayout {
                        visible: pitchModeTabs.currentIndex === 1
                        anchors.left: pitchModeTabs.right
                        anchors.leftMargin: Theme.spacingLarge
                        anchors.verticalCenter: pitchModeTabs.verticalCenter
                        spacing: Theme.spacingXSmall
                        z: 3

                        ToolModeButton {
                            id: handTool
                            ButtonGroup.group: pitchToolMode
                            checked: true
                            glyph: "\ue925"
                            ToolTip.visible: hovered
                            ToolTip.text: window.translator.tr("tool.hand") + " (V)"
                        }
                        ToolModeButton {
                            id: penTool
                            ButtonGroup.group: pitchToolMode
                            glyph: "\ue3c9"
                            ToolTip.visible: hovered
                            ToolTip.text: window.translator.tr("tool.pen") + " (P)"
                        }
                    }
                    Shortcut {
                        sequence: "V"
                        enabled: editorSplitView.extendedPitchEditorVisible
                        onActivated: handTool.checked = true
                    }
                    Shortcut {
                        sequence: "P"
                        enabled: editorSplitView.extendedPitchEditorVisible
                        onActivated: penTool.checked = true
                    }
                    DocumentFrame {
                        id: pitchEditorSurface
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        anchors.top: pitchModeTabs.bottom
                        anchors.topMargin: -overlap
                        contentMargin: frameWidth

                        StackLayout {
                            id: pitchModeStack
                            anchors.fill: parent
                            currentIndex: pitchModeTabs.currentIndex

                            ColumnLayout {
                                spacing: 0
                            PitchEditor {
                                id: pitchEditor
                                objectName: "pitchEditor"
                                Layout.fillWidth: true
                                Layout.fillHeight: true
                                Layout.leftMargin: Theme.spacing
                                Layout.rightMargin: Theme.spacing
                                Layout.topMargin: Theme.spacing
                                Layout.bottomMargin: basicPitchScrollBar.visible ? 0 : Theme.spacing
                                translator: window.translator
                                defaultMoraDuration: window.appBackend.defaultMoraDuration
                                defaultPauseDuration: window.appBackend.defaultPauseDuration
                                onPointsEdited: points => window.updatePitchPoints(points)
                                onTimingEdited: (durations, positions) =>
                                        window.updateMoraTiming(durations, positions)
                            }
                            PitchHorizontalScrollBar {
                                id: basicPitchScrollBar
                                Layout.fillWidth: true
                                editor: pitchEditor
                            }
                        }

                            Item {
                            PhonemeEditor {
                            id: phonemeEditor
                            anchors.fill: parent
                            anchors.margins: Theme.spacing
                            translator: window.translator
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
            }
                PlaybackControls {
                    Layout.fillWidth: true
                    Layout.preferredHeight: Theme.touchTarget
                    Layout.leftMargin: Theme.spacingLarge
                    Layout.rightMargin: Theme.spacingLarge
                    Layout.bottomMargin: Theme.spacingLarge - (Theme.touchTarget - Theme.largeButton) / 2
                    translator: window.translator
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
