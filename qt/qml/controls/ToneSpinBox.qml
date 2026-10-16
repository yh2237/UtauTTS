import QtQuick
import QtQuick.Controls

// 音高の入力。最小値は指定なし（自動）で、ほかはMIDIノート番号を音名で表示する。
SpinBox {
    id: root
    property string automaticText: ""
    readonly property int automaticValue: 35
    signal toneModified(string tone)

    from: automaticValue
    to: 84
    editable: true

    function toneToValue(tone) {
        const match = /^\s*([A-Ga-g])([#b]?)(-?\d+)\s*$/.exec(String(tone || ""));
        if (!match)
            return automaticValue;
        const semitones = {C: 0, D: 2, E: 4, F: 5, G: 7, A: 9, B: 11};
        let value = semitones[match[1].toUpperCase()] + (Number(match[3]) + 1) * 12;
        if (match[2] === "#")
            value += 1;
        else if (match[2] === "b")
            value -= 1;
        return Math.max(from, Math.min(to, value));
    }

    function valueToTone(value) {
        if (value <= automaticValue)
            return "";
        const names = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"];
        return names[value % 12] + (Math.floor(value / 12) - 1);
    }

    textFromValue: value => value <= automaticValue ? automaticText : valueToTone(value)
    valueFromText: text => toneToValue(text)
    validator: RegularExpressionValidator {
        regularExpression: /.*/
    }
    Binding {
        target: root.contentItem
        property: "text"
        value: root.textFromValue(root.value, root.locale)
    }
    onValueModified: toneModified(valueToTone(value))
}
