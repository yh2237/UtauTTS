pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ApplicationWindow {
    id: root
    required property var hostWindow
    required property var hostPalette
    required property var translator

    title: root.translator.tr("usage.title")
    visible: false
    width: 860
    height: 620
    minimumWidth: 860
    maximumWidth: 860
    minimumHeight: 620
    maximumHeight: 620
    transientParent: hostWindow
    modality: Qt.ApplicationModal
    flags: Qt.Dialog
    palette: hostPalette
    color: palette.window

    property int currentPage: 0
    property var pageNames: [
        root.translator.tr("usage.page.intro"),
        root.translator.tr("usage.page.textInput"),
        root.translator.tr("usage.page.utterance"),
        root.translator.tr("usage.page.playback"),
        root.translator.tr("usage.page.parameters"),
        root.translator.tr("usage.page.project"),
        root.translator.tr("usage.page.pitch"),
        root.translator.tr("usage.page.dictionary"),
        root.translator.tr("usage.page.settings"),
        root.translator.tr("usage.page.update")
    ]

    Component {
        id: usageStepRow
        RowLayout {
            id: stepRow
            required property var modelData
            spacing: 8
            Layout.fillWidth: true
            Label {
                text: stepRow.modelData.number + "."
                color: palette.highlight
                font.bold: true
            }
            Label {
                Layout.fillWidth: true
                text: stepRow.modelData.text
                wrapMode: Text.Wrap
            }
        }
    }

    RowLayout {
        anchors.fill: parent
        anchors.margins: 12
        spacing: 12

        ListView {
            id: usageNavigation
            Layout.preferredWidth: 170
            Layout.fillHeight: true
            clip: true
            model: root.pageNames
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

        StackLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            currentIndex: root.currentPage

            ScrollView {
                id: introPage
                contentWidth: availableWidth
                ColumnLayout {
                    width: introPage.availableWidth
                    spacing: 12
                    Label {
                        text: root.translator.tr("usage.page.intro")
                        font.bold: true
                        font.pixelSize: 20
                    }
                    Label {
                        Layout.fillWidth: true
                        text: root.translator.tr("usage.intro.description")
                        wrapMode: Text.Wrap
                    }
                }
            }

            ScrollView {
                id: textInputPage
                contentWidth: availableWidth
                ColumnLayout {
                    width: textInputPage.availableWidth
                    spacing: 12
                    Label {
                        text: root.translator.tr("usage.page.textInput")
                        font.bold: true
                        font.pixelSize: 20
                    }
                    Repeater {
                        model: [
                            { number: 1, text: root.translator.tr("usage.step.textInput.1") },
                            { number: 2, text: root.translator.tr("usage.step.textInput.2") },
                            { number: 3, text: root.translator.tr("usage.step.textInput.3") }
                        ]
                        delegate: usageStepRow
                    }
                }
            }

            ScrollView {
                id: utterancePage
                contentWidth: availableWidth
                ColumnLayout {
                    width: utterancePage.availableWidth
                    spacing: 12
                    Label {
                        text: root.translator.tr("usage.page.utterance")
                        font.bold: true
                        font.pixelSize: 20
                    }
                    Repeater {
                        model: [
                            { number: 1, text: root.translator.tr("usage.step.utterance.1") },
                            { number: 2, text: root.translator.tr("usage.step.utterance.2") },
                            { number: 3, text: root.translator.tr("usage.step.utterance.3") },
                            { number: 4, text: root.translator.tr("usage.step.utterance.4") }
                        ]
                        delegate: usageStepRow
                    }
                }
            }

            ScrollView {
                id: playbackPage
                contentWidth: availableWidth
                ColumnLayout {
                    width: playbackPage.availableWidth
                    spacing: 12
                    Label {
                        text: root.translator.tr("usage.page.playback")
                        font.bold: true
                        font.pixelSize: 20
                    }
                    Repeater {
                        model: [
                            { number: 1, text: root.translator.tr("usage.step.playback.1") },
                            { number: 2, text: root.translator.tr("usage.step.playback.2") },
                            { number: 3, text: root.translator.tr("usage.step.playback.3") },
                            { number: 4, text: root.translator.tr("usage.step.playback.4") }
                        ]
                        delegate: usageStepRow
                    }
                }
            }

            ScrollView {
                id: parameterPage
                contentWidth: availableWidth
                ColumnLayout {
                    width: parameterPage.availableWidth
                    spacing: 12
                    Label {
                        text: root.translator.tr("usage.page.parameters")
                        font.bold: true
                        font.pixelSize: 20
                    }
                    Repeater {
                        model: [
                            { number: 1, text: root.translator.tr("usage.step.parameters.1") },
                            { number: 2, text: root.translator.tr("usage.step.parameters.2") },
                            { number: 3, text: root.translator.tr("usage.step.parameters.3") },
                            { number: 4, text: root.translator.tr("usage.step.parameters.4") },
                            { number: 5, text: root.translator.tr("usage.step.parameters.5") },
                            { number: 6, text: root.translator.tr("usage.step.parameters.6") }
                        ]
                        delegate: usageStepRow
                    }
                }
            }

            ScrollView {
                id: projectPage
                contentWidth: availableWidth
                ColumnLayout {
                    width: projectPage.availableWidth
                    spacing: 12
                    Label {
                        text: root.translator.tr("usage.page.project")
                        font.bold: true
                        font.pixelSize: 20
                    }
                    Repeater {
                        model: [
                            { number: 1, text: root.translator.tr("usage.step.project.1") },
                            { number: 2, text: root.translator.tr("usage.step.project.2") },
                            { number: 3, text: root.translator.tr("usage.step.project.3") },
                            { number: 4, text: root.translator.tr("usage.step.project.4") }
                        ]
                        delegate: usageStepRow
                    }
                }
            }

            ScrollView {
                id: pitchPage
                contentWidth: availableWidth
                ColumnLayout {
                    width: pitchPage.availableWidth
                    spacing: 12
                    Label {
                        text: root.translator.tr("usage.page.pitch")
                        font.bold: true
                        font.pixelSize: 20
                    }
                    Label {
                        Layout.fillWidth: true
                        text: root.translator.tr("usage.pitch.description")
                        wrapMode: Text.Wrap
                    }
                    Repeater {
                        model: [
                            { number: 1, text: root.translator.tr("usage.step.pitch.1") },
                            { number: 2, text: root.translator.tr("usage.step.pitch.2") },
                            { number: 3, text: root.translator.tr("usage.step.pitch.3") },
                            { number: 4, text: root.translator.tr("usage.step.pitch.4") },
                            { number: 5, text: root.translator.tr("usage.step.pitch.5") }
                        ]
                        delegate: usageStepRow
                    }
                }
            }

            ScrollView {
                id: dictionaryPage
                contentWidth: availableWidth
                ColumnLayout {
                    width: dictionaryPage.availableWidth
                    spacing: 12
                    Label {
                        text: root.translator.tr("usage.page.dictionary")
                        font.bold: true
                        font.pixelSize: 20
                    }
                    Repeater {
                        model: [
                            { number: 1, text: root.translator.tr("usage.step.dictionary.1") },
                            { number: 2, text: root.translator.tr("usage.step.dictionary.2") },
                            { number: 3, text: root.translator.tr("usage.step.dictionary.3") }
                        ]
                        delegate: usageStepRow
                    }
                }
            }

            ScrollView {
                id: settingsPage
                contentWidth: availableWidth
                ColumnLayout {
                    width: settingsPage.availableWidth
                    spacing: 12
                    Label {
                        text: root.translator.tr("usage.page.settings")
                        font.bold: true
                        font.pixelSize: 20
                    }
                    Repeater {
                        model: [
                            { number: 1, text: root.translator.tr("usage.step.settings.1") },
                            { number: 2, text: root.translator.tr("usage.step.settings.2") },
                            { number: 3, text: root.translator.tr("usage.step.settings.3") },
                            { number: 4, text: root.translator.tr("usage.step.settings.4") },
                            { number: 5, text: root.translator.tr("usage.step.settings.5") }
                        ]
                        delegate: usageStepRow
                    }
                }
            }

            ScrollView {
                id: updatePage
                contentWidth: availableWidth
                ColumnLayout {
                    width: updatePage.availableWidth
                    spacing: 12
                    Label {
                        text: root.translator.tr("usage.page.update")
                        font.bold: true
                        font.pixelSize: 20
                    }
                    Repeater {
                        model: [
                            { number: 1, text: root.translator.tr("usage.step.update.1") }
                        ]
                        delegate: usageStepRow
                    }
                }
            }
        }
    }
}
