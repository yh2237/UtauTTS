package native

import (
	"context"
	"testing"
)

func TestCancelSynthesisStopsOnlyTheRunningCall(t *testing.T) {
	engine := &Engine{ctx: context.Background()}
	engine.cancelSynthesis()

	ctx, end := engine.beginSynthesis()
	engine.cancelSynthesis()
	if ctx.Err() == nil {
		t.Fatal("running synthesis was not canceled")
	}
	end()

	next, endNext := engine.beginSynthesis()
	defer endNext()
	if next.Err() != nil {
		t.Fatal("cancellation leaked into the next synthesis")
	}
}

func TestCancelSynthesisStopsOverlappingCalls(t *testing.T) {
	engine := &Engine{ctx: context.Background()}
	first, endFirst := engine.beginSynthesis()
	second, endSecond := engine.beginSynthesis()
	defer endSecond()
	endFirst()
	if first.Err() == nil {
		t.Fatal("finished synthesis kept its context")
	}
	engine.cancelSynthesis()
	if second.Err() == nil {
		t.Fatal("synthesis that outlived an earlier one was not canceled")
	}
}
