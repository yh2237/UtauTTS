package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yh2237/UtauTTS/internal/api"
	"github.com/yh2237/UtauTTS/internal/appinfo"
)

func localListenAddress(host string, port int) (string, error) {
	if !api.IsLoopbackHost(host) {
		return "", errors.New("UtauTTS Server is local-only; --host must be localhost or a loopback IP address")
	}
	if strings.EqualFold(host, "localhost") {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	var port int
	var host string
	var showVersion bool
	var writeTimeout time.Duration
	var config api.Config
	flag.BoolVar(&showVersion, "version", false, "print application version")
	flag.IntVar(&port, "port", 8080, "port")
	flag.StringVar(&host, "host", "127.0.0.1", "local listen host (localhost or a loopback IP address)")
	flag.StringVar(&config.VoiceDir, "voice-dir", "voice", "directory containing voicebanks")
	flag.StringVar(&config.OpenJTalkPath, "openjtalk-engine", "", "path to the Open JTalk engine library")
	flag.StringVar(&config.OpenJTalkDictionary, "openjtalk-dictionary", "", "path to Open JTalk dictionary")
	flag.IntVar(&config.MaxConcurrentSynthesis, "max-concurrent-synthesis", 4, "concurrent synthesis and analysis limit")
	flag.IntVar(&config.MaxConcurrentBatches, "max-concurrent-batches", 2, "concurrent batch limit")
	flag.DurationVar(&config.QueueTimeout, "queue-timeout", 30*time.Second, "how long a request waits for a free slot before 429; negative waits until the client disconnects")
	flag.DurationVar(&writeTimeout, "write-timeout", 10*time.Minute, "response write timeout")
	flag.Func("model-dir", "prosody model directory (repeatable)", func(value string) error {
		config.ModelDirectories = append(config.ModelDirectories, value)
		return nil
	})
	flag.Parse()
	if showVersion {
		fmt.Printf("%s %s\n", appinfo.Name(), appinfo.Version())
		return nil
	}
	if config.MaxConcurrentSynthesis < 1 || config.MaxConcurrentBatches < 1 {
		return errors.New("--max-concurrent-synthesis and --max-concurrent-batches must be at least 1")
	}
	if writeTimeout <= 0 {
		return errors.New("--write-timeout must be positive")
	}
	address, err := localListenAddress(host, port)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()
	apiServer, err := api.New(config)
	if err != nil {
		return err
	}
	defer func() { runErr = errors.Join(runErr, apiServer.Close()) }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	url := "http://" + listener.Addr().String()
	fmt.Printf("UTAUTTS_READY=%s\n", url)
	log.Printf("listening on %s", url)
	server := &http.Server{Handler: apiServer.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: writeTimeout, IdleTimeout: 60 * time.Second}
	server.BaseContext = func(net.Listener) context.Context { return ctx }
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	select {
	case err := <-served:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := server.Shutdown(shutdown)
		if err != nil {
			err = errors.Join(err, server.Close())
		}
		<-served
		return err
	}
}
