package main

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/tensor"
	"utautts/cmd/tools/internal/toolutil"
)

type parityFixture struct {
	IDs    [][3]int     `json:"ids"`
	Cont   [][4]float32 `json:"cont"`
	Output [][]float32  `json:"output"`
}

func writeFixture(path string, model *autograd.SpeechTiming, device tensor.Device) error {
	rng := rand.New(rand.NewSource(3))
	f := parityFixture{IDs: make([][3]int, 37), Cont: make([][4]float32, 37), Output: make([][]float32, 37)}
	ids := make([]int, 37*3)
	cv := make([]float32, 37*4)
	for i := range ids {
		ids[i] = rng.Intn(40)
		f.IDs[i/3][i%3] = ids[i]
	}
	for i := range cv {
		cv[i] = float32(rng.NormFloat64())
		f.Cont[i/4][i%4] = cv[i]
	}
	input, e := autograd.New(cv, []int{1, 37, 4}, device, false)
	if e != nil {
		return e
	}
	defer input.Close()
	wasTraining := model.Module.Training
	model.Train(false)
	defer model.Train(wasTraining)
	var pred *autograd.Tensor
	autograd.NoGrad(func() { pred = model.Forward(ids, input, 0) })
	values, e := pred.ToHost()
	pred.ReleaseGraph()
	if e != nil {
		return e
	}
	for i := range f.Output {
		f.Output[i] = append([]float32(nil), values[i*80:(i+1)*80]...)
	}
	b, e := json.Marshal(f)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	out, e := toolutil.CreateExclusive(path)
	if e != nil {
		return e
	}
	_, e = out.Write(b)
	closeErr := out.Close()
	if e != nil {
		return e
	}
	return closeErr
}
