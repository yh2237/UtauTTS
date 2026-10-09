package voicebank

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"
)

func TestDiscoverVoicebanksUsesMetadataNameAndSorts(t *testing.T) {
	root := t.TempDir()
	makeBank := func(directory, name string) {
		dir := filepath.Join(root, directory)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "character.txt"), []byte("name="+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	makeBank("second", "乙")
	makeBank("first", "甲")
	if err := os.Mkdir(filepath.Join(root, "not-a-bank"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "乙" || got[1].Name != "甲" {
		t.Fatalf("voicebanks = %#v", got)
	}
}

func TestDiscoverDiffSingerWithoutOto(t *testing.T) {
	root := t.TempDir()
	named := filepath.Join(root, "diffsinger")
	minimal := filepath.Join(root, "Minimal")
	for _, singer := range []string{named, minimal} {
		if err := os.Mkdir(singer, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(singer, "dsconfig.yaml"), []byte("acoustic: acoustic.onnx\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(named, "character.txt"), []byte("name=DS Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, summary := range got {
		if summary.Kind != "diffsinger" {
			t.Fatalf("voicebanks = %#v", got)
		}
		names[summary.Path] = summary.Name
	}
	if len(got) != 2 || names[named] != "DS Test" || names[minimal] != "Minimal" {
		t.Fatalf("voicebanks = %#v", got)
	}
}

func TestDiscoverDiffSingerBundleIgnoresCoreModules(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "Lewy")
	singer := filepath.Join(bundle, "Lewisia")
	core := filepath.Join(bundle, "0_CORE")
	for _, path := range []string{
		filepath.Join(singer, "dsconfig.yaml"),
		filepath.Join(core, "dsacoustic", "dsconfig.yaml"),
		filepath.Join(core, "dsdur", "dsconfig.yaml"),
		filepath.Join(core, "dspitch", "dsconfig.yaml"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("phonemes: phonemes.txt\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(singer, "character.txt"), []byte("name=Lewisia\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != singer || got[0].Name != "Lewisia" || got[0].Kind != "diffsinger" {
		t.Fatalf("voicebanks = %#v", got)
	}
}

func TestInspectFindsSafeImageAndPresentationText(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "portrait.bmp"), []byte("BM"), 0o644); err != nil {
		t.Fatal(err)
	}
	character, _ := japanese.ShiftJIS.NewEncoder().Bytes([]byte("name=試験音源\nimage=portrait.bmp\n"))
	readme, _ := japanese.ShiftJIS.NewEncoder().Bytes([]byte("これは説明です。\n"))
	if err := os.WriteFile(filepath.Join(root, "character.txt"), character, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.TXT"), readme, 0o644); err != nil {
		t.Fatal(err)
	}
	summary, err := Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Name != "試験音源" || summary.ImagePath != filepath.Join(root, "portrait.bmp") || summary.ReadmePath == "" {
		t.Fatalf("summary = %#v", summary)
	}
	presentation, err := LoadPresentation(summary)
	if err != nil || !strings.Contains(presentation.ReadmeText, "説明") || !strings.Contains(presentation.CharacterText, "image=") {
		t.Fatalf("presentation = %#v, %v", presentation, err)
	}
}

func TestInspectRejectsImageOutsideVoicebank(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "bank")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "outside.bmp"), []byte("BM"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "character.txt"), []byte("image=../outside.bmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	summary, err := Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ImagePath != "" {
		t.Fatalf("unsafe image path accepted: %q", summary.ImagePath)
	}
}

func TestDiscoverAcceptsVoicebankRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(root)
	if err != nil || len(got) != 1 || got[0].Path != root {
		t.Fatalf("Discover() = %#v, %v", got, err)
	}
}

func TestDiscoverFindsOneLevelWrappedVoicebank(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "bundle", "bundle")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "character.txt"), []byte("name=二重ルート音源\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != inner || got[0].Name != "二重ルート音源" {
		t.Fatalf("Discover() = %#v", got)
	}
}

func TestDiscoverFindsMetadataRootInsideTwoWrappers(t *testing.T) {
	root := t.TempDir()
	bankRoot := filepath.Join(root, "archive", "voice-library")
	otoRoot := filepath.Join(bankRoot, "english")
	if err := os.MkdirAll(otoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bankRoot, "character.txt"), []byte("name=Teto English\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otoRoot, "oto.ini"), []byte("a.wav=- h@,0,0,0,0,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(root)
	if err != nil || len(got) != 1 || got[0].Path != bankRoot || got[0].Name != "Teto English" {
		t.Fatalf("Discover() = %#v, %v", got, err)
	}
}

func TestInspectFindsNestedOtoWithoutParsingIt(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "append")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "oto.ini"), []byte("this is not a valid oto line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Inspect(root)
	if err != nil || got.Path != root {
		t.Fatalf("Inspect() = %#v, %v", got, err)
	}
}
