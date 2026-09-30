// Goとの受け渡しは [{unit_index: 0, pitch_factor: 1.2}, ...]。配列位置を音素番号にしない。
function normalize(values) {
    const result = [];
    for (let index = 0; index < (values || []).length; ++index) {
        const value = values[index];
        if (!value || typeof value !== "object")
            continue;
        const unitIndex = value.unit_index === undefined ? index : Number(value.unit_index);
        if (!Number.isInteger(unitIndex) || unitIndex < 0)
            continue;
        const copy = {unit_index: unitIndex};
        for (const key of Object.keys(value)) {
            if (key !== "unit_index" && value[key] !== null && value[key] !== undefined)
                copy[key] = value[key];
        }
        if (Object.keys(copy).length > 1)
            result.push(copy);
    }
    return result;
}

function update(values, unitIndex, key, value) {
    const result = normalize(values);
    if (!Number.isInteger(Number(unitIndex)) || Number(unitIndex) < 0 || !String(key || "")
            || key === "unit_index" || value === undefined || value === null
            || (typeof value === "number" && !Number.isFinite(value)))
        return result;
    let entry = result.find(item => item.unit_index === Number(unitIndex));
    if (!entry) {
        entry = {unit_index: Number(unitIndex)};
        result.push(entry);
    }
    entry[String(key)] = value;
    return result;
}

function remove(values, unitIndex) {
    return normalize(values).filter(item => item.unit_index !== Number(unitIndex));
}
