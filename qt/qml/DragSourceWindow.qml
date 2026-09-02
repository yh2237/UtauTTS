pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Window {
    id: root
    required property var hostPalette
    required property var backend
    required property var translator
    required property var files
    required property url exportDirectory
    required property bool ready
    required property color accent
    required property color mutedText
    signal dragError()

    title: root.translator.tr("drag.title")
    visible: false
    width: 720
    height: 420
    minimumWidth: 720
    maximumWidth: 720
    minimumHeight: 420
    maximumHeight: 420
    modality: Qt.NonModal
    flags: Qt.Window
    palette: hostPalette
    color: palette.window

    property bool exoDrag: root.files.length === 1 && String(root.files[0]).toLowerCase().endsWith(".exo")

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        Label {
            Layout.fillWidth: true
            text: root.exoDrag
                  ? root.translator.tr("drag.aviutlHint")
                  : root.translator.tr("drag.timelineHint")
            wrapMode: Text.WordWrap
        }

        Rectangle {
            id: dragSourceArea
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.minimumHeight: 130
            color: palette.alternateBase
            border.width: 2
            border.color: root.accent

            Label {
                anchors.centerIn: parent
                text: root.translator.tr("drag.timelineLabel")
                horizontalAlignment: Text.AlignHCenter
                color: palette.text
            }

            MouseArea {
                id: dragSourceMouseArea
                anchors.fill: parent
                enabled: root.ready && !root.backend.busy
                property point pressPosition: Qt.point(0, 0)
                property bool pressActive: false

                onPressed: mouse => {
                    pressPosition = Qt.point(mouse.x, mouse.y);
                    pressActive = true;
                }
                onPositionChanged: mouse => {
                    if (!pressActive)
                        return;
                    const dx = mouse.x - pressPosition.x;
                    const dy = mouse.y - pressPosition.y;
                    if (Math.sqrt(dx * dx + dy * dy) < 8)
                        return;
                    pressActive = false;
                    if (!root.backend.startFileDrag(root.files))
                        root.dragError();
                }
                onReleased: pressActive = false
                onCanceled: pressActive = false
            }
        }

        Label {
            Layout.fillWidth: true
            text: root.translator.tr("drag.saveDestination", root.exportDirectory.toString())
            elide: Text.ElideMiddle
            color: root.mutedText
        }

        RowLayout {
            Layout.fillWidth: true

            Item { Layout.fillWidth: true }

            Button {
                text: root.translator.tr("common.close")
                onClicked: root.close()
            }
        }
    }
}
