package hubert

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Configは音声エンコーダの形。HuBERT Base以外（XLS-R、WavLM）を読み分けるために使う。
// 値は重みと同じディレクトリのconfig.jsonから読む。読めなければHuBERT Baseとして扱う。
type Config struct {
	HiddenSize   int    `json:"hidden_size"`
	Layers       int    `json:"num_hidden_layers"`
	Heads        int    `json:"num_attention_heads"`
	Intermediate int    `json:"intermediate_size"`
	ConvDim      []int  `json:"conv_dim"`
	ConvKernel   []int  `json:"conv_kernel"`
	ConvStride   []int  `json:"conv_stride"`
	ConvBias     bool   `json:"conv_bias"`
	FeatNorm     string `json:"feat_extract_norm"`
	Stable       bool   `json:"do_stable_layer_norm"`
	PosKernel    int    `json:"num_conv_pos_embeddings"`
	PosGroups    int    `json:"num_conv_pos_embedding_groups"`
	ModelType    string `json:"model_type"`
	// RelativeBiasは、注意に相対位置のバイアスを足すか（WavLM）。
	RelativeBias bool `json:"-"`
	// GroupNormGroupsは、畳み込み後の正規化を何グループで行うか。0ならチャンネルごと（HuBERT）。
	GroupNormGroups int `json:"-"`
}

// DefaultConfigはHuBERT Baseの形を返す。
func DefaultConfig() Config {
	return Config{
		HiddenSize: 768, Layers: 12, Heads: 12, Intermediate: 3072,
		ConvDim:    []int{512, 512, 512, 512, 512, 512, 512},
		ConvKernel: []int{10, 3, 3, 3, 3, 2, 2},
		ConvStride: []int{5, 2, 2, 2, 2, 2, 2},
		ConvBias:   false, FeatNorm: "group",
		PosKernel: 128, PosGroups: 16, ModelType: "hubert",
	}
}

// loadConfigは、重みと同じディレクトリのconfig.jsonを読む。HuBERTのディレクトリには
// 別のconfig.jsonがあるため、エンコーダの設定として読めるものだけを使う。
func loadConfig(weightsPath string) Config {
	config := DefaultConfig()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(weightsPath), "config.json"))
	if err != nil {
		return config
	}
	var parsed Config
	if err := json.Unmarshal(data, &parsed); err != nil {
		return config
	}
	if parsed.HiddenSize <= 0 || parsed.Layers <= 0 || parsed.Heads <= 0 {
		return config
	}
	if parsed.Intermediate <= 0 {
		parsed.Intermediate = parsed.HiddenSize * 4
	}
	if len(parsed.ConvKernel) == 0 || len(parsed.ConvStride) == 0 {
		parsed.ConvKernel = config.ConvKernel
		parsed.ConvStride = config.ConvStride
	}
	if len(parsed.ConvDim) == 0 {
		parsed.ConvDim = config.ConvDim
	}
	if parsed.FeatNorm == "" {
		parsed.FeatNorm = config.FeatNorm
	}
	if parsed.PosKernel <= 0 {
		parsed.PosKernel = config.PosKernel
	}
	if parsed.PosGroups <= 0 {
		parsed.PosGroups = config.PosGroups
	}
	if parsed.ModelType == "wavlm" {
		// WavLMは事後正規化の層を使い、注意に相対位置のバイアスを足す。
		// 畳み込み後の正規化はチャンネルごと（HuBERTと同じ）。
		parsed.RelativeBias = true
		parsed.Stable = false
		parsed.GroupNormGroups = 0
	}
	return parsed
}

// baseHuBERTは、HuBERT Baseの形かどうかを返す。GPUの実装はHuBERT Base専用なので、それ以外はCPUで計算する。
func (c Config) baseHuBERT() bool {
	return c.HiddenSize == 768 && c.Layers == 12 && c.Heads == 12 && c.Intermediate == 3072 &&
		!c.Stable && c.FeatNorm == "group" && c.ModelType == "hubert"
}
