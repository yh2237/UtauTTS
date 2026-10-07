package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

func hmacSHA(key []byte, value string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(value))
	return h.Sum(nil)
}
func signR2(req *http.Request, bodyHash, keyID, secret string, now time.Time) {
	date := now.UTC().Format("20060102")
	timestamp := now.UTC().Format("20060102T150405Z")
	req.Header.Set("X-Amz-Date", timestamp)
	req.Header.Set("X-Amz-Content-Sha256", bodyHash)
	headerNames := []string{"host"}
	for name := range req.Header {
		headerNames = append(headerNames, strings.ToLower(name))
	}
	sort.Strings(headerNames)
	var canonicalHeaders strings.Builder
	for _, name := range headerNames {
		value := req.Header.Get(name)
		if name == "host" {
			value = req.URL.Host
		}
		canonicalHeaders.WriteString(name + ":" + strings.Join(strings.Fields(value), " ") + "\n")
	}
	signed := strings.Join(headerNames, ";")
	scope := date + "/auto/s3/aws4_request"
	canonical := strings.Join([]string{req.Method, req.URL.EscapedPath(), req.URL.RawQuery, canonicalHeaders.String(), signed, bodyHash}, "\n")
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + timestamp + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	k := hmacSHA([]byte("AWS4"+secret), date)
	k = hmacSHA(k, "auto")
	k = hmacSHA(k, "s3")
	k = hmacSHA(k, "aws4_request")
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+keyID+"/"+scope+", SignedHeaders="+signed+", Signature="+hex.EncodeToString(hmacSHA(k, toSign)))
}
func r2Request(client *http.Client, method, address string, body []byte, item asset, keyID, secret string) (*http.Response, error) {
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	req, err := http.NewRequest(method, address, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if method == "PUT" {
		req.Header.Set("Content-Type", item.ContentType)
		req.Header.Set("Cache-Control", immutableCache)
		req.Header.Set("X-Amz-Meta-Sha256", item.SHA256)
	}
	signR2(req, hash, keyID, secret, time.Now())
	return client.Do(req)
}
func sendR2(client *http.Client, method, address string, body []byte, item asset, keyID, secret string) (*http.Response, error) {
	var last error
	for attempt := 0; attempt < 5; attempt++ {
		resp, err := r2Request(client, method, address, body, item, keyID, secret)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			err = fmt.Errorf("R2 %s failed: %s: HTTP %d", method, item.Key, resp.StatusCode)
			if resp.StatusCode < 500 && resp.StatusCode != 429 {
				return nil, err
			}
		}
		last = err
		if attempt < 4 {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
	}
	return nil, last
}
func uploadOne(client *http.Client, output, bucket, account, keyID, secret string, item asset) error {
	file, err := checkedFile(filepath.Join(output, "r2"), item.Path)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	if int64(len(body)) != item.Size || hex.EncodeToString(sum[:]) != item.SHA256 {
		return fmt.Errorf("asset changed after packaging: %s", item.Path)
	}
	address := "https://" + account + ".r2.cloudflarestorage.com/" + url.PathEscape(bucket) + "/" + (&url.URL{Path: item.Key}).EscapedPath()
	for _, method := range []string{"PUT", "HEAD"} {
		requestBody := body
		if method == "HEAD" {
			requestBody = nil
		}
		resp, err := sendR2(client, method, address, requestBody, item, keyID, secret)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if method == "HEAD" && (resp.ContentLength != item.Size || resp.Header.Get("X-Amz-Meta-Sha256") != item.SHA256) {
			return fmt.Errorf("R2 verification failed: %s", item.Key)
		}
	}
	return nil
}
func upload(output, bucket, account string) error {
	if bucket == "" || account == "" {
		return fmt.Errorf("R2 bucket and Cloudflare account id are required")
	}
	keyID, secret := os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY")
	if keyID == "" || secret == "" {
		return fmt.Errorf("R2 access key and secret are required")
	}
	m, err := loadManifest(output)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	jobs := make(chan asset)
	failures := make(chan error, len(m.Assets))
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				failures <- uploadOne(client, output, bucket, account, keyID, secret, item)
			}
		}()
	}
	for _, item := range m.Assets {
		jobs <- item
	}
	close(jobs)
	workers.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			return err
		}
	}
	fmt.Printf("Uploaded and verified %d R2 objects in %s\n", len(m.Assets), m.R2Prefix)
	return nil
}
