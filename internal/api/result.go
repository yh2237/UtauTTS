package api

import (
	"net/http"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/synth"
)

type synthesisResponse struct {
	Result synth.Report `json:"result"`
	Audio  struct {
		Format string `json:"format"`
		Data   []byte `json:"data"`
	} `json:"audio"`
}

func (s *Server) handleSynthesizeResult(w http.ResponseWriter, r *http.Request) {
	var request SynthesisRequest
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeAPIError(w, jsonDecodeStatus(err), err)
		return
	}
	result, status, err := s.synthesize(r.Context(), request)
	if err != nil {
		writeAPIError(w, status, err)
		return
	}
	response := synthesisResponse{Result: result.Report()}
	response.Audio.Format = "wav_pcm_s16le"
	response.Audio.Data = audio.PCMToWavBytes(result.Audio)
	writeJSON(w, http.StatusOK, response)
}
