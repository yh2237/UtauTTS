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
    property alias currentIndex: voicebankDetailsList.currentIndex
    property var selectedVoicebank: root.backend.voicebanks.length && voicebankDetailsList.currentIndex >= 0 && voicebankDetailsList.currentIndex < root.backend.voicebanks.length ? root.backend.voicebanks[voicebankDetailsList.currentIndex] : null

    title: root.translator.tr("voicebankDetails.title")
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

    RowLayout {
        anchors.fill: parent
        anchors.margins: 10
        spacing: 8

        ListView {
            id: voicebankDetailsList
            Layout.preferredWidth: 210
            Layout.fillHeight: true
            clip: true
            model: root.backend.voicebanks
            currentIndex: 0

            delegate: ItemDelegate {
                required property int index
                required property var modelData
                width: ListView.view.width
                text: modelData.name
                highlighted: ListView.isCurrentItem
                onClicked: voicebankDetailsList.currentIndex = index
            }
        }

        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 10

            Label {
                Layout.fillWidth: true
                text: root.selectedVoicebank ? root.selectedVoicebank.name : ""
                font.pixelSize: 18
                font.bold: true
            }
            Label {
                Layout.fillWidth: true
                text: {
                    if (!root.selectedVoicebank)
                        return "";
                    const counts = root.selectedVoicebank.alias_counts || {};
                    return root.translator.tr("voicebankDetails.capabilities",
                                              counts["CV"] || 0,
                                              counts["VCV"] || 0,
                                              counts["VC"] || 0);
                }
                wrapMode: Text.Wrap
                color: palette.mid
            }
            Label {
                Layout.fillWidth: true
                text: "readme.txt"
                font.bold: true
            }
            ScrollView {
                id: voicebankReadmeScroll
                Layout.fillWidth: true
                Layout.fillHeight: true
                contentWidth: availableWidth
                Label {
                    width: voicebankReadmeScroll.availableWidth
                    text: root.selectedVoicebank ? (root.selectedVoicebank.readme_text || root.translator.tr("voicebankDetails.noReadme")) : ""
                    wrapMode: Text.Wrap
                    padding: 4
                }
            }
        }
    }
}
