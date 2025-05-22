package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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

	h := &BaseHandle{}
	http.Handle("/", h)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.LocalIP, config.Port),
		Handler: h,
	}

	log.Fatal(server.ListenAndServe())
}

type BaseHandle struct{}

var regexRaceHost = regexp.MustCompile(`^([a-z\-]+\.)?race\.gs\.`)
var regexSakeHost = regexp.MustCompile(`^([a-z\-]+\.)?sake\.gs\.`)
var regexGamestatsHost = regexp.MustCompile(`^([a-z\-]+\.)?gamestats2?\.gs\.`)
var regexStage1URL = regexp.MustCompile(`^/w([0-9])$`)

// This needs to be kept up to date with the info in wfc-servers' nas/main.go
var checks = []func(r *http.Request) (string, bool){
	// Check for *.sake.gs.* or sake.gs.*
	func(r *http.Request) (string, bool) {
		return "*.sake.gs.* or sake.gs.*", regexSakeHost.MatchString(r.Host)
	},

	// Check for *.gamestats(2).gs.* or gamestats(2).gs.*
	func(r *http.Request) (string, bool) {
		return "*.gamestats(2).gs.* or gamestats(2).gs.*", regexGamestatsHost.MatchString(r.Host)
	},

	// Check for *.race.gs.* or race.gs.*
	func(r *http.Request) (string, bool) {
		return "*.race.gs.* or race.gs.*", regexRaceHost.MatchString(r.Host)
	},

	// Handle conntest server
	func(r *http.Request) (string, bool) {
		return "conntest", strings.HasPrefix(r.Host, "conntest.")
	},

	// Handle DWC auth requests
	func(r *http.Request) (string, bool) {
		urlstr := r.URL.String()
		return "dwc auth", urlstr == "/ac" || urlstr == "/pr" || urlstr == "/download"
	},

	// Handle /nastest.jsp
	func(r *http.Request) (string, bool) {
		return "nastest.jsp", r.URL.Path == "/nastest.jsp"
	},

	// Check for /payload
	func(r *http.Request) (string, bool) {
		return "payload", strings.HasPrefix(r.URL.String(), "/payload")
	},

	// Handle stage1
	func(r *http.Request) (string, bool) {
		return "stage1", regexStage1URL.FindStringSubmatch(r.URL.String()) != nil
	},

	// Handle /api/*
	func(r *http.Request) (string, bool) {
		return "api", strings.HasPrefix(r.URL.Path, "/api")
	},
}

func shouldServeWFC(r *http.Request) (string, bool) {
	// Skip checks if the domain isn't correct
	if !strings.Contains(r.Host, config.HostDomain) {
		return fmt.Sprintf("Request failed to match rule for 'contains %s'", config.HostDomain), false
	}

	for _, check := range checks {
		if rule, success := check(r); success {
			return fmt.Sprintf("Request matched rule for '%s'", rule), true
		}
	}

	return "Request matched no rules", false
}

func (h *BaseHandle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status, shouldWFC := shouldServeWFC(r)

	referrer := r.Referer()
	if len(referrer) == 0 {
		referrer = "-"
	}

	userAgent := r.UserAgent()
	if len(userAgent) == 0 {
		userAgent = "-"
	}

	ctime := time.Now()
	_, offset := ctime.Zone()
	timeStr := fmt.Sprintf("%d/%s/%d:%d:%d:%d %d",
		ctime.Day(),
		ctime.Month().String(),
		ctime.Year(),
		ctime.Hour(),
		ctime.Minute(),
		ctime.Second(),
		offset/36,
	)

	fmt.Printf(
		"%s [%s] \"%s %s %s\"  \"%s\" \"%s\": %s\n",
		r.RemoteAddr,
		timeStr,
		r.Method,
		r.URL.String(),
		r.Proto,
		referrer,
		userAgent,
		status,
	)

	if shouldWFC {
		wfcProxy.ServeHTTP(w, r)
	} else {
		defaultProxy.ServeHTTP(w, r)
	}
}
