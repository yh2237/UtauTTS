package main

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPythonSubsetParity(t *testing.T){
	root:=filepath.Join("..","..","..","out","python-migration");read:=func(dir string)([]item,error){b,e:=os.ReadFile(filepath.Join(root,dir,"manifest.json"));if e!=nil{return nil,e};var rows []item;e=json.Unmarshal(b,&rows);return rows,e}
	a,e:=read("libritts-go");if e!=nil{t.Skipf("optional Go fixture: %v",e)};b,e:=read("libritts-py");if e!=nil{t.Skipf("optional Python fixture: %v",e)}
	if len(a)!=2||len(b)!=2{t.Fatalf("rows %d/%d",len(a),len(b))}
	for i:=range a{if a[i].ID!=b[i].ID||a[i].Speaker!=b[i].Speaker||a[i].Text!=b[i].Text||a[i].Duration!=b[i].Duration{t.Fatalf("row %d differs",i)};x,e:=os.ReadFile(a[i].AudioPath);if e!=nil{t.Fatal(e)};y,e:=os.ReadFile(b[i].AudioPath);if e!=nil{t.Fatal(e)};if sha256.Sum256(x)!=sha256.Sum256(y){t.Fatalf("WAV %s differs",a[i].ID)}}
	t.Logf("%d real LibriTTS-R utterances: metadata and WAV hashes match Python",len(a))
}
