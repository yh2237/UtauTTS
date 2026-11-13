pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

GridLayout {
    id: content
    required property var backend
    required property var translator
    readonly property bool compact: width < 600
    property alias currentIndex: voicebankDetailsList.currentIndex
    property var selectedVoicebank: content.backend.voicebanks.length
                                     && voicebankDetailsList.currentIndex >= 0
                                     && voicebankDetailsList.currentIndex < content.backend.voicebanks.length
                                     ? content.backend.voicebanks[voicebankDetailsList.currentIndex] : null

    anchors.margins: Theme.spacingLarge
    columns: compact ? 1 : 2
    columnSpacing: Theme.spacingLarge
    rowSpacing: Theme.spacingLarge

    SideList {
        id: voicebankDetailsList
        Layout.preferredWidth: content.compact ? -1 : Theme.sidebarWidth
        Layout.preferredHeight: content.compact ? Theme.touchTarget : -1
        Layout.fillWidth: content.compact
        Layout.fillHeight: !content.compact
        compact: content.compact
        model: content.backend.voicebanks
        textRole: "name"
        currentIndex: 0
        onClicked: index => voicebankDetailsList.currentIndex = index
    }

    ColumnLayout {
        Layout.fillWidth: true
        Layout.fillHeight: true
        spacing: Theme.spacingLarge

        Label {
            Layout.fillWidth: true
            text: content.selectedVoicebank ? content.selectedVoicebank.name : ""
            font.pixelSize: Theme.fontTitle
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
            color: Theme.inactiveText
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
            ScrollBar.horizontal.policy: ScrollBar.AlwaysOff
            // 出入りで幅が変わると折り返しと高さが循環するので、縦のスクロールバーは常に出す。
            ScrollBar.vertical.policy: ScrollBar.AlwaysOn
            TextArea {
                width: voicebankReadmeScroll.availableWidth
                text: content.selectedVoicebank ? (content.selectedVoicebank.readme_text || content.translator.tr("voicebankDetails.noReadme")) : ""
                textFormat: TextEdit.PlainText
                readOnly: true
                selectByMouse: true
                wrapMode: TextEdit.Wrap
            }
        }
    }
}
