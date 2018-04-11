package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/pelletier/go-toml"
)

var (
	config = flag.String("config", "./config.toml", "path to config file")
)

func printTemplateConfig() {
	c := Config{
		Server: ServerConfig{
			Address: "0.0.0.0:8080",
			Store:   "bolt", // bolt or in-memory
			RegistrationTimeoutStr: "240h",
			MaxMessages:            6,         // 0 is unlimited
			MaxLength:              10 * 1024, // [bytes] 0 us unlimited
			CommunicateTimeoutStr:  "15s",
			Certificate: &CertificateConfig{
				KeyFile:  "/path/to/key",
				CertFile: "/path/to/cert",
			},
		},
		Bolt: &BoltConfig{
			"path/to/boltdb",
		},
	}

	c.Bolt.File = "/path/to/boltdb"
	b, _ := toml.Marshal(c)

	fmt.Println(string(b))
}

func main() {
	flag.Parse()

	fp, err := os.Open(*config)

	if err != nil {
		log.Println("Opening config error:", err)

		os.Exit(1)
	}

	if err := toml.NewDecoder(fp).Decode(&globalConfig); err != nil {
		log.Println("Decoding config error:", err)

		os.Exit(1)
	}

	if err := validateConfig(&globalConfig); err != nil {
		log.Println(err)

		os.Exit(1)
	}

	handler, server, err := initHandler(&globalConfig)

	if err != nil {
		log.Fatalf("newHandler error: %v", err)
	}

	listener, err := net.Listen("tcp", globalConfig.Server.Address)

	if err != nil {
		log.Fatalf("port opening error: %v", err)
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh,
			syscall.SIGTERM,
			syscall.SIGINT)

		for {
			switch <-sigCh {
			case syscall.SIGTERM, syscall.SIGINT, os.Interrupt:
				server.GracefulStop()

				if handler.KV != nil {
					handler.KV.Stop()
				}

				return
			}
		}
	}()

	log.Println("Listening started")

	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
