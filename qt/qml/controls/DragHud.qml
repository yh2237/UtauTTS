pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls

// ドラッグ中の値をカーソル横に出す共通部品。位置は使う側が指定する。
Rectangle {
    id: root
    property string hudText: ""

    visible: hudText.length > 0
    width: hudLabel.implicitWidth + 16
    height: hudLabel.implicitHeight + 8
    radius: 4
    color: "#ee222222"

    Label {
        id: hudLabel
        anchors.centerIn: parent
        text: root.hudText
        color: "white"
        font.pixelSize: 12
    }
}
