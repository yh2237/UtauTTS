// Package neural は特定providerに依存しないニューラル合成の入力DTOと契約を定義する。
package neural

import (
	"context"
	"errors"
	"sync"

	"utautts/internal/audio"
	"utautts/internal/engine"
	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

// Inputはニューラル合成providerが共通して必要とする解決済み入力。
// tts側がConfigから変換し、provider実装はこのDTOだけを参照する。
type Input struct {
	Context context.Context
	// VoicebankPathは音源ディレクトリ。
	VoicebankPath string
	// TextとToneは計画へ記録する元入力。
	Text string
	Tone string
	// Language/Phonemizer/Reading/Moraeは発音解析の結果。
	Language   string
	Phonemizer string
	Reading    string
	Morae      []frontend.Mora
	// Features/MoraDurationsMS/PitchPointsはプロソディ予測の結果。
	Features        []prosody.FeatureFrame
	MoraDurationsMS []float64
	PitchPoints     []float64
	// PitchCurveはprovider向けのピッチ曲線。自動生成か手動かをAutomaticPitchで示す。
	PitchCurve     *render.PitchCurve
	AutomaticPitch bool
	// PhoneWeightsはモーラごとの音素時間配分の重み。
	PhoneWeights [][]float64
	// ProviderOptionsは選択provider固有の設定。
	ProviderOptions render.ProviderOptions
	// Engineは解決済みrenderer。provider resourceの参照に使う。
	Engine engine.ResolvedEngine
}

// Outputはニューラル合成の低層結果。tts側でResultへ変換する。
type Output struct {
	Plan            *plan.Plan
	Audio           *audio.PCM
	MoraDurationsMS []float64
	MoraPositionsMS []float64
	PitchPoints     []float64
}

// Synthesizerはニューラル歌唱スコアから音声を構築するprovider契約。
type Synthesizer interface {
	ProviderID() engine.ProviderID
	Synthesize(Input) (*Output, error)
}

// Factoryはprovider実装を組み立てる。登録側が実装型に依存せずに済む。
type Factory func() Synthesizer

// レジストリはprovider IDから実装を引く。登録はinitや起動時にRegisterで行う。
var (
	mu           sync.RWMutex
	synthesizers = map[engine.ProviderID]Factory{}
)

// Registerはprovider実装をprovider IDで登録する。同一IDの再登録は上書きする。
func Register(id engine.ProviderID, factory Factory) {
	if id == "" || factory == nil {
		panic("neural: synthesizer registration requires a provider id and factory")
	}
	mu.Lock()
	defer mu.Unlock()
	synthesizers[id] = factory
}

// ForProviderは登録済みproviderを解決する。未登録はfound=falseを返す。
func ForProvider(id engine.ProviderID) (Synthesizer, bool) {
	mu.RLock()
	factory, found := synthesizers[id]
	mu.RUnlock()
	if !found {
		return nil, false
	}
	return factory(), true
}

var (
	closerMu sync.Mutex
	closers  []func() error
)

// RegisterCloserは常駐するproviderセッションの解放関数を登録する。providerアダプターのinitから呼ぶ。
func RegisterCloser(fn func() error) {
	if fn == nil {
		return
	}
	closerMu.Lock()
	closers = append(closers, fn)
	closerMu.Unlock()
}

// CloseSessionsは登録済みの常駐providerセッションを登録と逆順に解放する。
func CloseSessions() error {
	closerMu.Lock()
	registered := append([]func() error(nil), closers...)
	closerMu.Unlock()
	var err error
	for index := len(registered) - 1; index >= 0; index-- {
		err = errors.Join(err, registered[index]())
	}
	return err
}
