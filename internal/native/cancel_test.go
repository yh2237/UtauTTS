package native

import (
	"context"
	"testing"
)

func TestCancelSynthesisStopsOnlyTheRunningCall(t *testing.T) {
	engine := &Engine{ctx: context.Background()}
	engine.cancelSynthesis()

	ctx, cancel := engine.beginSynthesis()
	engine.cancelSynthesis()
	if ctx.Err() == nil {
		t.Fatal("running synthesis was not canceled")
	}
	engine.endSynthesis(cancel)

	next, nextCancel := engine.beginSynthesis()
	defer engine.endSynthesis(nextCancel)
	if next.Err() != nil {
		t.Fatal("cancellation leaked into the next synthesis")
	}
}
