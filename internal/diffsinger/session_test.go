package diffsinger

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/engine"
	"utautts/internal/provider"
)

func runDiffSingerProviderHelper() {
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(provider.Hello{
		Type: provider.MessageHello, Protocol: provider.ProtocolName, ProtocolVersion: provider.ProtocolVersion,
		Provider: "diffsinger", ProviderVersion: "1", Session: true,
		Capabilities: []string{"frame_pitch", provider.CapabilityNeuralScoreJobV1},
		Contracts:    []provider.ContractSupport{{Name: "neural-synthesizer", Version: 1}},
	}); err != nil {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	count := int16(0)
	for scanner.Scan() {
		var header struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(scanner.Bytes(), &header) != nil {
			return
		}
		switch header.Type {
		case provider.MessageRender:
			var request provider.RenderRequest
			if json.Unmarshal(scanner.Bytes(), &request) != nil {
				return
			}
			count++
			if err := audio.WriteWav(request.OutputPath, &audio.PCM{SampleRate: 44100, Channels: 1, Data: []int16{count}}); err != nil {
				return
			}
			if err := encoder.Encode(provider.Result{
				Type: provider.MessageResult, RequestID: request.RequestID,
				Audio: provider.AudioArtifact{Path: request.OutputPath, Format: "wav_pcm_s16le", SampleRate: 44100, Channels: 1},
			}); err != nil {
				return
			}
		case provider.MessageShutdown:
			return
		}
	}
}

func TestDiffSingerProviderSessionKeepsBridgeResident(t *testing.T) {
	t.Setenv("UTAUTTS_DIFFSINGER_PROVIDER_HELPER", "1")
	t.Cleanup(func() { _ = CloseProviderSessions() })
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	request := Request{
		Tokens: []int64{0}, Durations: []int64{2}, F0: []float32{261, 261}, SampleRate: 44100,
	}
	first, err := RenderSession(context.Background(), executable, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderSession(context.Background(), executable, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Data) != 1 || first.Data[0] != 1 || len(second.Data) != 1 || second.Data[0] != 2 {
		t.Fatalf("session outputs = %#v, %#v", first, second)
	}
}

func TestRenderSessionFallsBackToLegacyBridge(t *testing.T) {
	t.Setenv("UTAUTTS_DIFFSINGER_TEST_BRIDGE", "1")
	pcm, err := RenderSession(nil, os.Args[0], Request{
		Tokens: []int64{0}, Durations: []int64{2}, F0: []float32{261, 261}, SampleRate: 44100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pcm.SampleRate != 44100 || len(pcm.Data) != 2 {
		t.Fatalf("pcm = %#v", pcm)
	}
}

func TestWriteProviderRequestUsesCommonNeuralScoreJob(t *testing.T) {
	score := engine.NeuralScore{
		Symbols: []string{"SP", "a"}, Durations: []int64{2, 4}, F0: []float32{220, 220, 220, 220, 220, 220},
		MIDI: 60, WordDiv: []int64{2}, WordDur: []int64{6}, UsePitchPredictor: true,
	}
	directory, path, _, err := writeProviderRequest(score, Request{AcousticPath: "acoustic.onnx", VocoderPath: "vocoder.onnx"})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(directory)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var job provider.NeuralSynthesizerJob
	if err := json.Unmarshal(data, &job); err != nil {
		t.Fatal(err)
	}
	if job.Version != provider.NeuralSynthesizerJobVersion || job.Contract != "neural-synthesizer" ||
		job.ContractVersion != 1 || job.Score.MIDI != 60 || len(job.Score.Symbols) != 2 ||
		job.Resources["acoustic_model"] != "acoustic.onnx" {
		t.Fatalf("job = %#v", job)
	}
	var options Request
	if err := json.Unmarshal(job.Options, &options); err != nil {
		t.Fatal(err)
	}
	if options.Version != 1 || options.OutputPath == "" || options.AcousticPath != "" || options.VocoderPath != "" {
		t.Fatalf("options = %#v", options)
	}
}
