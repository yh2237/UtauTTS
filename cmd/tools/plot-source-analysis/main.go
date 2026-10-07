package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"utautts/cmd/tools/internal/sourcephone"
	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/audio"
)

type aliases []string

func (a *aliases) String() string     { return strings.Join(*a, ",") }
func (a *aliases) Set(v string) error { *a = append(*a, v); return nil }

type chart struct {
	x0, y0, x1, y1 int
	duration       float64
}

func (c chart) x(ms float64) int { return c.x0 + int(math.Round(ms/c.duration*float64(c.x1-c.x0))) }
func (c chart) y(value, min, max float64) int {
	return c.y1 - int(math.Round((value-min)/(max-min)*float64(c.y1-c.y0)))
}
func fill(img *image.RGBA, x0, y0, x1, y1 int, col color.RGBA) {
	draw.Draw(img, image.Rect(x0, y0, x1, y1), &image.Uniform{C: col}, image.Point{}, draw.Src)
}
func line(img *image.RGBA, x0, y0, x1, y1 int, col color.RGBA) {
	dx, dy := int(math.Abs(float64(x1-x0))), int(math.Abs(float64(y1-y0)))
	sx, sy := -1, -1
	if x0 < x1 {
		sx = 1
	}
	if y0 < y1 {
		sy = 1
	}
	err := dx - dy
	for {
		if image.Pt(x0, y0).In(img.Bounds()) {
			img.SetRGBA(x0, y0, col)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}
func frame(img *image.RGBA, c chart) {
	gray := color.RGBA{185, 190, 195, 255}
	fill(img, c.x0, c.y0, c.x1, c.y1, color.RGBA{250, 250, 250, 255})
	for i := 1; i < 5; i++ {
		x := c.x0 + (c.x1-c.x0)*i/5
		y := c.y0 + (c.y1-c.y0)*i/5
		line(img, x, c.y0, x, c.y1, gray)
		line(img, c.x0, y, c.x1, y, gray)
	}
	dark := color.RGBA{60, 65, 70, 255}
	line(img, c.x0, c.y0, c.x1, c.y0, dark)
	line(img, c.x0, c.y1, c.x1, c.y1, dark)
	line(img, c.x0, c.y0, c.x0, c.y1, dark)
	line(img, c.x1, c.y0, c.x1, c.y1, dark)
}
func series(img *image.RGBA, c chart, frames []any, key string, min, max float64, col color.RGBA) {
	prevX, prevY, have := 0, 0, false
	for _, raw := range frames {
		f := sourcephone.Map(raw)
		x := c.x(sourcephone.Number(f["start_ms"]))
		y := c.y(sourcephone.Number(f[key]), min, max)
		if have {
			line(img, prevX, prevY, x, y, col)
		}
		prevX, prevY, have = x, y, true
	}
}
func vline(img *image.RGBA, c chart, ms float64, col color.RGBA) {
	x := c.x(ms)
	line(img, x, c.y0, x, c.y1, col)
}
func shade(img *image.RGBA, c chart, start, end float64, col color.RGBA) {
	x0, x1 := c.x(start), c.x(end)
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if x0 < c.x0 {
		x0 = c.x0
	}
	if x1 > c.x1 {
		x1 = c.x1
	}
	if col.A == 255 {
		fill(img, x0, c.y0, x1, c.y1, col)
		return
	}
	a := float64(col.A) / 255
	for y := c.y0; y < c.y1; y++ {
		for x := x0; x < x1; x++ {
			old := img.RGBAAt(x, y)
			img.SetRGBA(x, y, color.RGBA{uint8(float64(old.R)*(1-a) + float64(col.R)*a), uint8(float64(old.G)*(1-a) + float64(col.G)*a), uint8(float64(old.B)*(1-a) + float64(col.B)*a), 255})
		}
	}
}
func spectrogram(img *image.RGBA, c chart, pcm *audio.PCM) {
	rate := pcm.SampleRate
	channels := pcm.Channels
	samples := len(pcm.Data) / channels
	nfft := int(math.Max(64, float64(rate)*.02))
	if nfft > samples {
		nfft = samples
	}
	if nfft < 2 {
		return
	}
	hop := int(math.Max(1, float64(rate)*.002))
	bins := c.y1 - c.y0
	cols := c.x1 - c.x0
	maxHz := math.Min(8000, float64(rate)/2)
	powers := make([][]float64, cols)
	peak := 1e-20
	for x := 0; x < cols; x++ {
		center := int(float64(x) / float64(cols) * float64(samples))
		start := center - nfft/2
		if start < 0 {
			start = 0
		}
		if start+nfft > samples {
			start = samples - nfft
		}
		start = (start / hop) * hop
		if start+nfft > samples {
			start = samples - nfft
		}
		row := make([]float64, bins)
		for y := 0; y < bins; y++ {
			hz := float64(bins-y) / float64(bins) * maxHz
			re, im := 0.0, 0.0
			for j := 0; j < nfft; j++ {
				v := 0.0
				for ch := 0; ch < channels; ch++ {
					v += float64(pcm.Data[(start+j)*channels+ch])
				}
				v /= float64(channels * 32768)
				phase := 2 * math.Pi * hz * float64(j) / float64(rate)
				re += v * math.Cos(phase)
				im -= v * math.Sin(phase)
			}
			power := (re*re + im*im) / float64(nfft*nfft)
			row[y] = power
			if power > peak {
				peak = power
			}
		}
		powers[x] = row
	}
	for x, row := range powers {
		for y, power := range row {
			db := 10 * math.Log10(math.Max(power, 1e-20)/peak)
			t := math.Max(0, math.Min(1, (db+70)/70))
			r := uint8(30 + 220*t)
			g := uint8(15 + 100*t*t)
			b := uint8(55 + 120*(1-t))
			img.SetRGBA(c.x0+x, c.y0+y, color.RGBA{r, g, b, 255})
		}
	}
}
func plot(reportPath string, selected []any, spans sourcephone.Object, detail bool) (*image.RGBA, error) {
	width, rowHeight := 1800, 450
	panels := 2
	if detail {
		width, rowHeight, panels = 2400, 525, 3
	}
	img := image.NewRGBA(image.Rect(0, 0, width, rowHeight*len(selected)))
	fill(img, 0, 0, width, img.Bounds().Dy(), color.RGBA{255, 255, 255, 255})
	for row, raw := range selected {
		unit := sourcephone.Map(raw)
		analysis := sourcephone.Map(unit["analysis"])
		duration := sourcephone.Number(analysis["duration_ms"])
		if duration <= 0 {
			return nil, fmt.Errorf("invalid source duration")
		}
		margin, gap := 80, 45
		panelWidth := (width - 2*margin - (panels-1)*gap) / panels
		charts := make([]chart, panels)
		for i := range charts {
			charts[i] = chart{x0: margin + i*(panelWidth+gap), y0: row*rowHeight + 60, x1: margin + i*(panelWidth+gap) + panelWidth, y1: (row+1)*rowHeight - 70, duration: duration}
			frame(img, charts[i])
		}
		blue := color.RGBA{22, 79, 114, 255}
		green := color.RGBA{44, 186, 153, 255}
		purple := color.RGBA{133, 49, 166, 255}
		ink := color.RGBA{45, 50, 55, 255}
		label(img, margin, row*rowHeight+23, sourcephone.String(unit["alias"]), ink, 2)
		for _, c := range charts {
			for tick := 0; tick <= 4; tick++ {
				ms := duration * float64(tick) / 4
				label(img, c.x(ms)-10, c.y1+12, fmt.Sprintf("%.0f", ms), ink, 2)
			}
			label(img, c.x0, c.y1+35, "ms after oto offset", ink, 2)
		}
		if detail {
			label(img, charts[0].x0, charts[0].y0-22, "source waveform", ink, 2)
			label(img, charts[1].x0, charts[1].y0-22, "spectrum khz", ink, 2)
			label(img, charts[2].x0, charts[2].y0-22, "rms dbfs", ink, 2)
			source := sourcephone.ResolvedSource(reportPath, sourcephone.String(unit["source_clip"]))
			digest, _, err := sourcephone.ClipIdentity(source)
			if err != nil {
				return nil, err
			}
			if digest != sourcephone.String(analysis["source_sha256"]) {
				return nil, fmt.Errorf("source PCM changed since observation")
			}
			pcm, err := audio.ReadWav(source)
			if err != nil {
				return nil, err
			}
			lastX, lastY, have := 0, 0, false
			for sample := 0; sample < len(pcm.Data)/pcm.Channels; sample += int(math.Max(1, float64(pcm.SampleRate)/4000)) {
				amplitude := 0.0
				for ch := 0; ch < pcm.Channels; ch++ {
					amplitude += float64(pcm.Data[sample*pcm.Channels+ch]) / float64(pcm.Channels*32768)
				}
				x := charts[0].x(float64(sample) * 1000 / float64(pcm.SampleRate))
				y := charts[0].y(amplitude, -1, 1)
				if have {
					line(img, lastX, lastY, x, y, blue)
				}
				lastX, lastY, have = x, y, true
			}
			spectrogram(img, charts[1], pcm)
			series(img, charts[2], sourcephone.List(analysis["frames"]), "rms_dbfs", -100, 0, blue)
			thresholdY := charts[2].y(sourcephone.Number(analysis["low_energy_threshold_dbfs"]), -100, 0)
			line(img, charts[2].x0, thresholdY, charts[2].x1, thresholdY, color.RGBA{128, 128, 128, 255})
			if spans != nil {
				span := sourcephone.Map(spans[fmt.Sprint(sourcephone.Int(unit["unit_index"]))])
				if span != nil {
					if span["source_sha256"] != digest || span["alignment_sha256"] != sourcephone.Map(unit["phone_alignment"])["alignment_sha256"] {
						return nil, fmt.Errorf("span source or alignment identity mismatch")
					}
					for _, c := range charts {
						shade(img, c, sourcephone.Number(span["core_start_ms"]), sourcephone.Number(span["core_end_ms"]), color.RGBA{44, 186, 153, 40})
					}
				}
			}
			for _, raw := range sourcephone.List(unit["forced_phone_intervals"]) {
				phone := sourcephone.Map(raw)
				for _, c := range charts {
					vline(img, c, sourcephone.Number(phone["start_ms"]), green)
					vline(img, c, sourcephone.Number(phone["end_ms"]), green)
					label(img, c.x((sourcephone.Number(phone["start_ms"])+sourcephone.Number(phone["end_ms"]))/2)-8, c.y0+8, sourcephone.String(phone["symbol"]), ink, 2)
				}
			}
		} else {
			label(img, charts[0].x0, charts[0].y0-22, "source level and landmarks", ink, 2)
			label(img, charts[1].x0, charts[1].y0-22, "acoustic evidence forced phones", ink, 2)
			for _, raw := range sourcephone.List(analysis["regions"]) {
				region := sourcephone.Map(raw)
				palette := map[string]color.RGBA{"periodic": {191, 228, 198, 255}, "aperiodic-high-crossing": {255, 211, 168, 255}, "mixed-or-uncertain": {255, 242, 194, 255}, "low-energy": {221, 221, 221, 255}}
				for _, c := range charts {
					shade(img, c, sourcephone.Number(region["start_ms"]), sourcephone.Number(region["end_ms"]), palette[sourcephone.String(region["evidence"])])
				}
			}
			frames := sourcephone.List(analysis["frames"])
			series(img, charts[0], frames, "rms_dbfs", -100, 0, blue)
			series(img, charts[1], frames, "periodicity", 0, 1.05, blue)
			series(img, charts[1], frames, "zero_crossing_rate", 0, 1.05, color.RGBA{162, 84, 21, 255})
			for _, raw := range sourcephone.List(unit["forced_phone_intervals"]) {
				phone := sourcephone.Map(raw)
				vline(img, charts[1], sourcephone.Number(phone["start_ms"]), purple)
				vline(img, charts[1], sourcephone.Number(phone["end_ms"]), purple)
				label(img, charts[1].x((sourcephone.Number(phone["start_ms"])+sourcephone.Number(phone["end_ms"]))/2)-8, charts[1].y0+8, sourcephone.String(phone["symbol"]), ink, 2)
			}
			for _, raw := range sourcephone.List(analysis["landmarks"]) {
				landmark := sourcephone.Map(raw)
				vline(img, charts[0], sourcephone.Number(landmark["source_ms"]), color.RGBA{38, 122, 160, 255})
			}
		}
	}
	return img, nil
}
func run(args []string) error {
	f := flag.NewFlagSet("plot-source-analysis", flag.ContinueOnError)
	reportPath := f.String("report", "", "")
	out := f.String("out", "", "")
	detail := f.Bool("detail", false, "")
	spansPath := f.String("spans", "", "")
	var filters aliases
	f.Var(&filters, "alias", "alias to plot (repeatable)")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *reportPath == "" || *out == "" {
		return fmt.Errorf("--report and --out required")
	}
	if err := toolutil.RequireUnderOut(*out, "output", false); err != nil {
		return err
	}
	if *spansPath != "" && !*detail {
		return fmt.Errorf("--spans requires --detail")
	}
	report, err := sourcephone.Read(*reportPath)
	if err != nil {
		return err
	}
	selected := []any{}
	for _, raw := range sourcephone.List(report["units"]) {
		unit := sourcephone.Map(raw)
		if len(filters) == 0 || containsAlias(filters, sourcephone.String(unit["alias"])) {
			selected = append(selected, raw)
		}
	}
	if len(selected) == 0 {
		return fmt.Errorf("no matching units")
	}
	var spans sourcephone.Object
	if *spansPath != "" {
		value, err := sourcephone.Read(*spansPath)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(*reportPath)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if value["observation_report_sha256"] != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("span observation report identity mismatch")
		}
		spans = sourcephone.Object{}
		for _, raw := range sourcephone.List(value["units"]) {
			u := sourcephone.Map(raw)
			spans[fmt.Sprint(sourcephone.Int(u["unit_index"]))] = u
		}
	}
	img, err := plot(*reportPath, selected, spans, *detail)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		return err
	}
	file, err := toolutil.CreateExclusive(*out)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}
func containsAlias(values []string, wanted string) bool {
	for _, v := range values {
		if v == wanted {
			return true
		}
	}
	return false
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
