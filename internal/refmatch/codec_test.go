package refmatch

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
)

// testdata/world_codec.jsonはpyworldのcode/decode_spectral_envelopeとcode/decode_aperiodicityの出力。
func TestCodecMatchesWorld(t *testing.T) {
	data, err := os.ReadFile("testdata/world_codec.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]struct {
		FFT int         `json:"fft"`
		CSP []float64   `json:"csp"`
		CAP [][]float64 `json:"cap"`
		DSP []float64   `json:"dsp"`
		DAP [][]float64 `json:"dap"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, rate := range []int{24000, 44100} {
		want := fixtures[strconv.Itoa(rate)]
		c := newCodec(rate, want.FFT, 60)
		bins := want.FFT/2 + 1
		sp := make([]float64, bins)
		ap := [][]float64{make([]float64, bins), make([]float64, bins)}
		for j := range sp {
			x := float64(j)
			sp[j] = math.Exp(-8 + 0.5*math.Sin(x*0.013) + 0.3*math.Cos(x*0.071))
			ap[0][j] = 0.5 + 0.45*math.Sin(x*0.02)
			ap[1][j] = 0.01
		}
		coded := make([]float64, 60)
		c.codeSpectrum(sp, coded)
		assertClose(t, rate, "coded spectrum", coded, want.CSP, 1e-9)
		decoded := make([]float64, bins)
		c.decodeSpectrum(want.CSP, decoded)
		assertClose(t, rate, "decoded spectrum", every(decoded, 37), want.DSP, 1e-9*want.DSP[0])
		for row := range ap {
			codedAP := make([]float64, c.apBands)
			c.codeAperiodicity(ap[row], codedAP)
			assertClose(t, rate, "coded aperiodicity", codedAP, want.CAP[row], 1e-9)
			c.decodeAperiodicity(want.CAP[row], decoded)
			assertClose(t, rate, "decoded aperiodicity", every(decoded, 37), want.DAP[row], 1e-9)
		}
	}
}

func every(values []float64, step int) []float64 {
	var out []float64
	for i := 0; i < len(values); i += step {
		out = append(out, values[i])
	}
	return out
}

func assertClose(t *testing.T, rate int, name string, got, want []float64, tolerance float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d %s: length %d, want %d", rate, name, len(got), len(want))
	}
	for i := range got {
		if math.Abs(got[i]-want[i]) > tolerance {
			t.Fatalf("%d %s[%d] = %g, want %g", rate, name, i, got[i], want[i])
		}
	}
}
