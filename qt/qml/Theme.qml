pragma Singleton

import QtQuick

QtObject {
    property bool dark: false

    readonly property color window: dark ? "#202020" : "#f3f3f3"
    readonly property color windowText: dark ? "#ffffff" : "#1a1a1a"
    readonly property color view: dark ? "#2d2d2d" : "#fbfbfb"
    readonly property color viewAlternate: dark ? "#393939" : "#f2f2f2"
    readonly property color viewText: dark ? "#ffffff" : "#1a1a1a"
    readonly property color button: dark ? "#2d2d2d" : "#fbfbfb"
    readonly property color buttonText: dark ? "#ffffff" : "#1a1a1a"
    readonly property color selection: "#d35f6b"
    readonly property color selectionText: "#ffffff"
    // Fusion の highlightedOutline（selection.darker(125)、明るさは HSL で 160 まで）。
    readonly property color focusOutline: "#c47c83"
    readonly property color selectionDisabled: dark ? "#434343" : "#bfbfbf"
    readonly property color placeholderText: dark ? "#cfcfcf" : "#5f5f5f"
    readonly property color inactiveText: dark ? "#cccccc" : "#5c5c5c"
    readonly property color disabledText: dark ? "#717171" : "#9b9b9b"
    readonly property color disabledButton: dark ? "#2a2a2a" : "#f5f5f5"
    readonly property color subtleHover: dark ? "#2d2d2d" : "#eaeaea"
    readonly property color buttonHover: dark ? "#323232" : "#f6f6f6"
    readonly property color buttonPressed: dark ? "#272727" : "#f5f5f5"
    readonly property color toolTipBase: dark ? "#2c2c2c" : "#f9f9f9"
    readonly property color toolTipText: dark ? "#ffffff" : "#1a1a1a"
    readonly property color light: dark ? "#454545" : "#ffffff"
    readonly property color midlight: dark ? "#2c2c2c" : "#f9f9f9"
    readonly property color border: dark ? "#424242" : "#c1c1c1"
    readonly property color shade: dark ? "#1c1c1c" : "#868686"
    readonly property color shadow: dark ? "#0a0a0a" : "#5c5c5c"
    readonly property color frameLine: Qt.lighter(Qt.darker(window, 1.4), 1.08)
    readonly property color axisLine: dark ? "#4a4a4a" : "#c5c5c5"

    readonly property color negative: dark ? "#ff99a4" : "#c42b1c"
    readonly property color negativeBackground: dark ? "#442726" : "#fde7e9"
    readonly property color negativeText: dark ? "#ffffff" : "#1a1a1a"

    readonly property color overlay: "#ee222222"
    readonly property color overlayText: "#ffffff"

    readonly property int fontXSmall: 10
    readonly property int fontSmall: 11
    readonly property int fontMedium: 13
    readonly property int fontLarge: 16
    readonly property int fontTitle: 20

    readonly property FontLoader iconFontLoader: FontLoader {
        source: "qrc:/fonts/MaterialSymbolsOutlined-subset.ttf"
    }
    readonly property string iconFont: iconFontLoader.name
    readonly property int iconSmall: 16
    readonly property int icon: 22
    readonly property int iconLarge: 28

    readonly property int spacingXSmall: 2
    readonly property int spacingSmall: 4
    readonly property int spacingMedium: 6
    readonly property int spacing: 8
    readonly property int spacingLarge: 12
    readonly property int spacingXLarge: 16

    readonly property int fieldWidthSmall: 96
    readonly property int fieldWidth: 180
    readonly property int fieldWidthLarge: 240
    readonly property int dialogWidth: 440
    readonly property int dialogWidthWide: 480
    readonly property int sidebarWidth: 200
    readonly property int cardHeight: 42
    readonly property int largeButton: 40
    readonly property int touchTarget: 48

    readonly property int radius: 2
    readonly property int radiusLarge: 4
}
