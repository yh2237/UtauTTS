// 原音エディタの表示値計算。QMLとテストで共有する。
function unitAt(units, index) {
    if (index < 0 || index >= units.length)
        return null;
    return units[index];
}

function overrideAt(overrides, index) {
    for (const value of overrides || []) {
        if (value && Number(value.unit_index) === Number(index))
            return value;
    }
    return null;
}

function unitValue(units, overrides, previewUnit, index, key) {
    if (previewUnit && Number(previewUnit.index) === Number(index)
            && String(previewUnit.key) === String(key))
        return previewUnit.value;
    const override = overrideAt(overrides, index);
    if (override && override[key] !== undefined)
        return override[key];
    const unit = unitAt(units, index);
    const overrideFlag = String(key) + "_override";
    if (String(key).indexOf("resampler_") === 0 && unit
            && unit[overrideFlag] !== true)
        return paramRange(key).def;
    return unit && unit[key] !== undefined ? unit[key] : paramRange(key).def;
}

function paramRange(key) {
    switch (String(key)) {
    case "pitch_factor":
    case "energy_factor":
        return {min: 0.1, max: 4.0, def: 1.0, isFloat: true};
    case "resampler_velocity":
    case "resampler_volume":
        return {min: 0, max: 200, def: 100, isFloat: false};
    case "resampler_modulation":
        return {min: 0, max: 100, def: 0, isFloat: false};
    case "resampler_tempo":
        return {min: 40, max: 300, def: 120, isFloat: true};
    case "consonant_ms":
    case "preutterance_ms":
    case "overlap_ms":
        return {min: 0, max: 1000, def: 0, isFloat: false};
    case "offset_ms":
    case "cutoff_ms":
        return {min: -2000, max: 5000, def: 0, isFloat: false};
    default:
        return {min: 0, max: 100, def: 0, isFloat: false};
    }
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {unitAt, overrideAt, unitValue, paramRange};
