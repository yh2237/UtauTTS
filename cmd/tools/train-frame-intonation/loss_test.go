package main

import (
	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/tensor"
	"math"
	"testing"
)

func TestSequenceLossPyTorchParity(t *testing.T) {
	pred, e := autograd.New([]float32{.2, .4, .8, -.3, 1.2, -.6}, []int{1, 6}, tensor.CPU, true)
	if e != nil {
		t.Fatal(e)
	}
	target, e := autograd.New([]float32{.8, .9, 1, .4, -.8, .1}, []int{1, 6}, tensor.CPU, false)
	if e != nil {
		t.Fatal(e)
	}
	mask := []bool{true, true, true, true, true, true}
	var owned []*autograd.Tensor
	loss := sequenceLoss(pred, target, mask, .35, -1, 1, tensor.CPU, &owned)
	value, e := loss.ToHost()
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(float64(value[0])-.6047500371932983) > 1e-6 {
		t.Fatalf("loss %.9f", value[0])
	}
	if e = loss.Backward(); e != nil {
		t.Fatal(e)
	}
	grad, e := pred.GradToHost()
	if e != nil {
		t.Fatal(e)
	}
	want := []float64{.042999982833862305, -.06399998813867569, .0560000017285347, -.18833333253860474, .30666667222976685, -.15333333611488342}
	for i, v := range grad {
		if math.Abs(float64(v)-want[i]) > 1e-5 {
			t.Errorf("gradient %d = %.9f want %.9f", i, v, want[i])
		}
	}
	loss.ReleaseGraph()
	for _, v := range owned {
		v.Close()
	}
	pred.Close()
	target.Close()
}
