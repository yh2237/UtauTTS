package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)
func TestPythonJSUTParity(t *testing.T){
	root:=filepath.Join("..","..","..","out","python-migration");a:=filepath.Join(root,"jsut-go");b:=filepath.Join(root,"jsut-py")
	for _,name:=range []string{"metadata.csv","wavs/BASIC5000_0001.wav","wavs/BASIC5000_0002.wav"}{x,e:=os.ReadFile(filepath.Join(a,name));if e!=nil{t.Skipf("optional Go corpus: %v",e)};y,e:=os.ReadFile(filepath.Join(b,name));if e!=nil{t.Skipf("optional Python corpus: %v",e)};if !bytes.Equal(x,y){t.Fatalf("%s differs from Python",name)}}
	t.Log("two real JSUT records: metadata and WAV files byte-identical")
}
