pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// 音源の詳細表示の共有ビュー。ウィンドウ装飾は shell 側が担当する。
RowLayout {
    id: content
    required property var backend
    required property var translator
    property alias currentIndex: voicebankDetailsList.currentIndex
    property var selectedVoicebank: content.backend.voicebanks.length
                                     && voicebankDetailsList.currentIndex >= 0
                                     && voicebankDetailsList.currentIndex < content.backend.voicebanks.length
                                     ? content.backend.voicebanks[voicebankDetailsList.currentIndex] : null

    anchors.margins: 10
    spacing: 8

    ListView {
        id: voicebankDetailsList
        Layout.preferredWidth: 210
        Layout.fillHeight: true
        clip: true
        model: content.backend.voicebanks
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
            text: content.selectedVoicebank ? content.selectedVoicebank.name : ""
            font.pixelSize: 18
            font.bold: true
        }
        Label {
            Layout.fillWidth: true
            text: {
                if (!content.selectedVoicebank)
                    return "";
                const counts = content.selectedVoicebank.alias_counts || {};
                return content.translator.tr("voicebankDetails.capabilities",
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
                text: content.selectedVoicebank ? (content.selectedVoicebank.readme_text || content.translator.tr("voicebankDetails.noReadme")) : ""
                wrapMode: Text.Wrap
                padding: 4
            }
        }
    }
}
