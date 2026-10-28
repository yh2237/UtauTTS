.pragma library
.import "QmlCore.js" as QmlCore
.import "MoraPositions.js" as MoraPositions

// sequenceは、発話の項目に入っているJSON配列を返す。
function sequence(item, name) {
    return QmlCore.decodeSequence(item ? item[name] : "[]");
}

// manualMoraDurationsは、手動で指定されたモーラの長さがあるかを返す。
function hasManualMoraDurations(item) {
    if (item && item.manualMoraDurationEdited)
        return true;
    return QmlCore.decodeSequence(item ? item.moraDurationsJson : "")
            .some(value => Number(value) > 0);
}

// displayedMoraDurationsは、表示に使うモーラの長さを返す。
function displayedMoraDurations(item) {
    if (hasManualMoraDurations(item))
        return QmlCore.decodeSequence(item.moraDurationsJson);
    return sequence(item, "autoMoraDurationsJson");
}

// displayedMoraPositionsは、表示に使うモーラの位置を返す。
function displayedMoraPositions(item) {
    if (hasManualMoraDurations(item))
        return MoraPositions.normalizedMoraPositions(
                QmlCore.decodeSequence(item.moraPositionsJson));
    return MoraPositions.normalizedMoraPositions(sequence(item, "autoMoraPositionsJson"));
}

// prosodyReadyは、抑揚のプレビューが揃っているかを返す。
function prosodyReady(item) {
    const moraCount = QmlCore.decodeSequence(item ? item.moraeJson : "[]").length;
    return moraCount > 0
            && sequence(item, "autoPointsJson").length === moraCount
            && sequence(item, "autoMoraDurationsJson").length === moraCount
            && sequence(item, "autoMoraPositionsJson").length === moraCount;
}
