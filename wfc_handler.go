package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

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

type WFCHandler struct{}

func (h *WFCHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		r.Host+r.URL.String(),
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
