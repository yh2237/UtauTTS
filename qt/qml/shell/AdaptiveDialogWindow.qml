pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import UtauTTS.Platform 1.0

// レイアウト切替でも内容を再生成せず、入力中の値を保持する。
ApplicationWindow {
    id: root
    required property var hostWindow
    property int dialogWidth: 720
    property int dialogHeight: 540
    property bool centerInHost: false
    readonly property bool compact: Platform.isWeb && Platform.isMobile
    width: dialogWidth
    height: dialogHeight
    minimumWidth: dialogWidth
    maximumWidth: dialogWidth
    minimumHeight: dialogHeight
    maximumHeight: dialogHeight
    transientParent: hostWindow
    modality: Qt.ApplicationModal
    flags: Platform.isWeb ? Qt.Dialog | Qt.WindowTitleHint | Qt.WindowCloseButtonHint : Qt.Dialog

    // 非表示のwasm窓にはハンドルがなく、サイズ更新で削除済みDOMへの参照が残りうる。
    Binding {
        target: root
        property: "minimumWidth"
        when: root.visible
        value: root.compact ? 0 : root.dialogWidth
        restoreMode: Binding.RestoreNone
    }
    Binding {
        target: root
        property: "maximumWidth"
        when: root.visible
        value: root.compact ? Math.max(1, root.hostWindow.width - 8) : root.dialogWidth
        restoreMode: Binding.RestoreNone
    }
    Binding {
        target: root
        property: "minimumHeight"
        when: root.visible
        value: root.compact ? 0 : root.dialogHeight
        restoreMode: Binding.RestoreNone
    }
    Binding {
        target: root
        property: "maximumHeight"
        when: root.visible
        value: root.compact ? Math.max(1, root.hostWindow.height - 32) : root.dialogHeight
        restoreMode: Binding.RestoreNone
    }
    Binding {
        target: root
        property: "width"
        when: root.visible
        value: root.compact ? Math.max(1, root.hostWindow.width - 8) : root.dialogWidth
        restoreMode: Binding.RestoreNone
    }
    Binding {
        target: root
        property: "height"
        when: root.visible
        value: root.compact ? Math.max(1, root.hostWindow.height - 32) : root.dialogHeight
        restoreMode: Binding.RestoreNone
    }

    Binding {
        target: root
        property: "x"
        when: root.visible && (root.compact || root.centerInHost)
        value: root.hostWindow.x + (root.hostWindow.width - root.width) / 2
        restoreMode: Binding.RestoreNone
    }
    Binding {
        target: root
        property: "y"
        when: root.visible && (root.compact || root.centerInHost)
        value: root.hostWindow.y + (root.hostWindow.height - root.height) / 2
        restoreMode: Binding.RestoreNone
    }
}
