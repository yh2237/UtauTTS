package settings

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
)

// Decodeは値を設定の型と範囲で確かめて返す。number・integerはfloat64、booleanはbool、enum・stringはstring。
func (setting Setting) Decode(raw json.RawMessage) (any, error) {
	var value any
	var err error
	switch setting.Type {
	case "number", "integer":
		var number float64
		err = json.Unmarshal(raw, &number)
		value = number
	case "boolean":
		var flag bool
		err = json.Unmarshal(raw, &flag)
		value = flag
	default:
		var text string
		err = json.Unmarshal(raw, &text)
		value = text
	}
	if err != nil {
		return nil, fmt.Errorf("setting %q expects %s", setting.ID, setting.Type)
	}
	return value, setting.Check(value)
}

// Checkは、Decodeが返す型の値が設定の範囲と選択肢に収まるかを確かめる。
func (setting Setting) Check(value any) error {
	switch value := value.(type) {
	case float64:
		if setting.Type == "integer" && value != math.Trunc(value) {
			return fmt.Errorf("setting %q expects integer", setting.ID)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) ||
			setting.Min != nil && value < *setting.Min || setting.Max != nil && value > *setting.Max {
			return fmt.Errorf("setting %q is outside the supported range: %v", setting.ID, value)
		}
	case string:
		if setting.Type == "enum" && !slices.ContainsFunc(setting.Options, func(option Option) bool { return option.Value == value }) {
			return fmt.Errorf("setting %q has no option %q", setting.ID, value)
		}
	}
	return nil
}
