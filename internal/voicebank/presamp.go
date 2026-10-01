package voicebank

import (
	"os"

	presampfile "github.com/yh2237/utauio/presamp"

	"utautts/internal/frontend"
)

type Presamp struct {
	Path         string
	Vowels       map[string]string
	Consonants   map[string]string
	Replacements map[string]string
	Endings      []string
}

func (p *Presamp) FrontendConfig() frontend.PresampConfig {
	if p == nil {
		return frontend.PresampConfig{}
	}
	return frontend.PresampConfig{
		Vowels: p.Vowels, Consonants: p.Consonants,
		Replacements: p.Replacements, Endings: p.Endings,
	}
}

func (b *Bank) loadPresamp(rootEntries ...[]os.DirEntry) {
	path := findRootFile(b.Root, "presamp.ini", rootEntries...)
	if path == "" {
		return
	}
	text, err := readMetadata(path)
	if err != nil {
		b.Diagnostics = append(b.Diagnostics, Diagnostic{Path: path, Message: err.Error()})
		return
	}
	config := presampfile.Parse(text)
	b.Presamp = &Presamp{Path: path, Vowels: config.Vowels, Consonants: config.Consonants,
		Replacements: config.Replacements, Endings: config.Endings}
}
