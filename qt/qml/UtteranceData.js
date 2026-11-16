.pragma library
.import "QmlCore.js" as QmlCore
.import "MoraPositions.js" as MoraPositions

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

function displayedMoraDurations(item) {
    if (hasManualMoraDurations(item))
        return QmlCore.decodeSequence(item.moraDurationsJson);
    return sequence(item, "autoMoraDurationsJson");
}

function displayedMoraPositions(item) {
    if (hasManualMoraDurations(item))
        return MoraPositions.normalizedMoraPositions(
                QmlCore.decodeSequence(item.moraPositionsJson));
    return MoraPositions.normalizedMoraPositions(sequence(item, "autoMoraPositionsJson"));
}
