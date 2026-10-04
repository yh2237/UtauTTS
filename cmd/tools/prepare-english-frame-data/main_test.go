package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func loadJSONL(t *testing.T,path string) []map[string]any {
	t.Helper();f,e:=os.Open(path);if e!=nil{t.Fatal(e)};defer f.Close();s:=bufio.NewScanner(f);s.Buffer(make([]byte,4096),16<<20);var rows []map[string]any;for s.Scan(){var row map[string]any;if e=json.Unmarshal(s.Bytes(),&row);e!=nil{t.Fatal(e)};rows=append(rows,row)};if e=s.Err();e!=nil{t.Fatal(e)};return rows
}
func TestPythonEnglishCorpusParity(t *testing.T) {
	root:=filepath.Join("..","..","..","out","python-migration")
	goPath:=filepath.Join(root,"english-go.jsonl");pyPath:=filepath.Join(root,"english-py.jsonl")
	if _,e:=os.Stat(pyPath);e!=nil{t.Skipf("optional Python parity output: %v",e)}
	a,b:=loadJSONL(t,goPath),loadJSONL(t,pyPath);if len(a)!=738||len(b)!=738{t.Fatalf("rows %d/%d",len(a),len(b))}
	for i:=range a{if !reflect.DeepEqual(a[i],b[i]){t.Fatalf("record %d differs: %v / %v",i,a[i]["id"],b[i]["id"])}}
	t.Logf("%d English training records structurally identical to Python",len(a))
}
func TestNormalizePhones(t *testing.T){for input,want:=range map[string]string{"AH0":"ah","ER1":"er","sp":"sp"}{if got:=normalize(input);got!=want{t.Fatalf("%s -> %s want %s",input,got,want)}}}
