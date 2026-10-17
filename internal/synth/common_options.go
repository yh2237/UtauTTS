package synth

// CommonOptionsは公開入力と既存入口に共通する設定。未指定の項目は変更しない。
type CommonOptions struct {
	MoraDurationMS, PauseDurationMS, IntonationStrength, ReleaseMS *float64
	ApplyPitch, ContextDuration, BoundaryTone, StretchAdapt        *bool
	PauseContext                                                   *bool
}

func ApplyCommonOptions(input Request, options CommonOptions) Request {
	for _, option := range []struct{ value, target *float64 }{
		{options.MoraDurationMS, &input.MoraDurationMS},
		{options.PauseDurationMS, &input.PauseDurationMS},
		{options.IntonationStrength, &input.IntonationStrength},
		{options.ReleaseMS, &input.ReleaseMS},
	} {
		if option.value != nil {
			*option.target = *option.value
		}
	}
	if options.PauseDurationMS != nil {
		input.PauseDurationSet = true
	}
	if options.ReleaseMS != nil {
		input.ReleaseSet = true
	}
	for _, option := range []struct{ value, target *bool }{
		{options.ApplyPitch, &input.ApplyPitch}, {options.ContextDuration, &input.ContextDuration},
		{options.BoundaryTone, &input.BoundaryTone}, {options.StretchAdapt, &input.StretchAdapt},
		{options.PauseContext, &input.PauseContext},
	} {
		if option.value != nil {
			*option.target = *option.value
		}
	}
	return input
}
