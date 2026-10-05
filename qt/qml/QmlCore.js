// QMLとテストで共有する純関数。QML JSのimportに依存せず、必要な値は引数で受け取る。
function decodeSequence(json) {
    if (!json || !json.length)
        return [];
    try {
        const value = JSON.parse(json);
        return Array.isArray(value) ? value : [];
    } catch (error) {
        return [];
    }
}

function copySequence(sequence) {
    const result = [];
    if (!sequence)
        return result;
    const size = sequence.length !== undefined ? sequence.length : sequence.count;
    for (let index = 0; index < size; ++index)
        result.push(sequence.get ? sequence.get(index) : sequence[index]);
    return result;
}

function currentUtterance(utterances, selectedIndex) {
    return utterances.count
            ? utterances.get(Math.max(0, Math.min(selectedIndex, utterances.count - 1))) : null;
}

function utteranceIndex(utterances, id) {
    for (let i = 0; i < utterances.count; ++i)
        if (utterances.get(i).utteranceId === id)
            return i;
    return -1;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {decodeSequence, copySequence, currentUtterance, utteranceIndex};
