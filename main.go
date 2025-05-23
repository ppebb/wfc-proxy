package main

import (
	"errors"
	"fmt"
	"log"
	"net/http/httputil"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
	"ppeb.me/wfc-proxy/nhttp"
)

type Config struct {
	LocalIP       string
	Port          int
	HostDomain    string
	WFCRemote     string
	DefaultRemote string
}

var (
	NoDomainSpecified        = errors.New("No Host Domain specified! Specify one to continue.")
	NoWFCRemoteSpecified     = errors.New("No WFC Remote specified in config! Specify one to continue.")
	NoDefaultRemoteSpecified = errors.New("No Default Remote specified in config! Specify one to continue.")
)

func readConfig(path string) (Config, error) {
	config := Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	if config.Port == 0 {
		config.Port = 80
	}

	if config.LocalIP == "" {
		config.LocalIP = "0.0.0.0"
	}

	fmt.Printf("Listening on %s:%d\n", config.LocalIP, config.Port)

	if len(config.HostDomain) == 0 {
		return config, NoDomainSpecified
	}

	if len(config.WFCRemote) == 0 {
		return config, NoWFCRemoteSpecified
	}

	if len(config.DefaultRemote) == 0 {
		return config, NoDefaultRemoteSpecified
	}

	return config, nil
}

func print_help() {
	fmt.Printf(`
ppeb's simple wfc proxy!!!

Usage: ./wfc-proxy [OPTIONS]
 -h|--help             Display this message and exit
 -c|--config           Specify the path to your config.yml
`)

	os.Exit(1)
}

var (
	config       Config
	wfcProxy     *httputil.ReverseProxy
	defaultProxy *httputil.ReverseProxy
)

func main() {
	var configPath string

	argsLen := len(os.Args)

	if argsLen <= 1 {
		fmt.Println("No arguments provided! --config is required to continue.")
		print_help()
	}

	for i := 1; i < argsLen; i++ {
		arg := os.Args[i]
		switch arg {
		case "-h", "--help":
			print_help()
		case "-c", "--config":
			if argsLen > i+1 {
				configPath = os.Args[i+1]
				i++
			}
		default:
			fmt.Printf("Unknown argument %s!\n", arg)
			print_help()
		}
	}

	if len(configPath) == 0 {
		log.Fatal("Missing config argument, provide a config.yml with -c or --config!")
	}

	config, err := readConfig(configPath)
	if err != nil {
		log.Fatal("Failed to read config: ", err.Error())
	}

	wfcRemote, err := url.Parse(config.WFCRemote)
	if err != nil {
		log.Fatal("Invalid WFC Remote provided: ", err.Error())
	}

	wfcProxy = httputil.NewSingleHostReverseProxy(wfcRemote)

	defaultRemote, err := url.Parse(config.DefaultRemote)
	if err != nil {
		log.Fatal("Invalid Default Remote provided: ", err.Error())
	}

	defaultProxy = httputil.NewSingleHostReverseProxy(defaultRemote)

	server := &nhttp.Server{
		Addr:    fmt.Sprintf("%s:%d", config.LocalIP, config.Port),
		Handler: &WFCHandler{},
	}

	log.Fatal(server.ListenAndServe())
}
