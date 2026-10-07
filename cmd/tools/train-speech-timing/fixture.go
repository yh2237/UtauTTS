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
	IDs          [][3]int                     `json:"ids"`
	Cont         [][4]float32                 `json:"cont"`
	F0Cont       [][f0ContextFeatures]float32 `json:"f0_cont,omitempty"`
	Output       [][]float32                  `json:"output"`
	F0Output     []float32                    `json:"f0_output,omitempty"`
	EnergyOutput []float32                    `json:"energy_output,omitempty"`
}

func writeFixture(path string, model trainerModel, device tensor.Device, phones int) error {
	rng := rand.New(rand.NewSource(3))
	f := parityFixture{IDs: make([][3]int, 37), Cont: make([][4]float32, 37), Output: make([][]float32, 37)}
	ids := make([]int, 37*3)
	cv := make([]float32, 37*4)
	f0cv := make([]float32, 37*f0ContextFeatures)
	f.F0Cont = make([][f0ContextFeatures]float32, 37)
	for i := range ids {
		ids[i] = rng.Intn(phones)
		f.IDs[i/3][i%3] = ids[i]
	}
	for i := range cv {
		cv[i] = float32(rng.NormFloat64())
		f.Cont[i/4][i%4] = cv[i]
	}
	for i := range f0cv {
		f0cv[i] = float32(rng.NormFloat64())
		f.F0Cont[i/f0ContextFeatures][i%f0ContextFeatures] = f0cv[i]
	}
	input, e := autograd.New(cv, []int{1, 37, 4}, device, false)
	if e != nil {
		return e
	}
	defer input.Close()
	f0Input, e := autograd.New(f0cv, []int{1, 37, f0ContextFeatures}, device, false)
	if e != nil {
		return e
	}
	defer f0Input.Close()
	wasTraining := model.module().Training
	model.train(false)
	defer model.train(wasTraining)
	var pred, f0Pred, energyPred *autograd.Tensor
	autograd.NoGrad(func() { pred, f0Pred, energyPred = model.forward(ids, input, f0Input, 0) })
	values, e := pred.ToHost()
	pred.ReleaseGraph()
	if e != nil {
		return e
	}
	for i := range f.Output {
		f.Output[i] = append([]float32(nil), values[i*80:(i+1)*80]...)
	}
	if f0Pred != nil {
		f0Values, e := f0Pred.ToHost()
		f0Pred.ReleaseGraph()
		if e != nil {
			return e
		}
		f.F0Output = f0Values
	}
	if energyPred != nil {
		energyValues, e := energyPred.ToHost()
		energyPred.ReleaseGraph()
		if e != nil {
			return e
		}
		f.EnergyOutput = energyValues
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
