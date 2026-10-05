.pragma library

function copySequence(sequence) {
    const result = [];
    if (!sequence)
        return result;
    const size = sequence.length !== undefined ? sequence.length : sequence.count;
    for (let index = 0; index < size; ++index)
        result.push(sequence.get ? sequence.get(index) : sequence[index]);
    return result;
}

// 保存形式の中心時刻を開始時刻へ戻す。nullは保つ。
function normalizedMoraPositions(positions) {
    const normalized = copySequence(positions);
    if (!normalized.length)
        return normalized;
    if (normalized[0] === null || normalized[0] === undefined)
        return normalized;
    const first = Number(normalized[0]);
    if (!Number.isFinite(first))
        return normalized;
    const origin = Math.max(0, first);
    for (let index = 0; index < normalized.length; ++index) {
        if (normalized[index] === null || normalized[index] === undefined)
            continue;
        const value = Number(normalized[index]);
        if (Number.isFinite(value))
            normalized[index] = Math.max(0, value - origin);
    }
    normalized[0] = 0;
    return normalized;
}

function moraStartsFromCenters(centers, durations) {
    const starts = [];
    const size = centers ? centers.length : 0;
    for (let index = 0; index < size; ++index) {
        const center = Number(centers[index]);
        const duration = index < (durations ? durations.length : 0) ? Number(durations[index]) : 0;
        const start = Number.isFinite(center) && Number.isFinite(duration) && duration > 0
                ? center - duration / 2 : null;
        starts.push(start !== null && start >= 0 ? start : null);
    }
    return normalizedMoraPositions(starts);
}
