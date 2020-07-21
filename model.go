package main

import (
	"sync"
)

// A single entry to "ping"
type Ping struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Url     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// A response
type Pong struct {
	Ping     Ping   `json:"ping"`
	Time     int64  `json:"time"`
	Response string `json:"response"`
}

// All responses
type Pongs struct {
	Path      string   `json:"path"`
	Method    string   `json:"method"`
	Time      int64    `json:"time"`
	Urls      []string `json:"urls"`
	Responses []string `json:"responses"`
}

// Pre-parse the input to see if it is an openapi 3.0 or swagger 2.0 file
type SwaggerOpenApi struct {
	Swagger string `json:"swagger;omitempty"`
	OpenAPI string `json:"openapi;omitempty"`
}

// The default request headers
var Headers = map[string]string{
	"Accept":       "*/*",
	"Connection":   "Keep-Alive",
	"Content-Type": "application/json",
	"User-Agent":   "aPing",
}

// A pool of Ping objects to reduce the GC overhead
var pingPool = sync.Pool{
	New: func() interface{} {
		return new(Ping)
	},
}

// A pool of Pong objects to reduce the GC overhead
var pongPool = sync.Pool{
	New: func() interface{} {
		return new(Pong)
	},
}

// All collected Pongs
var Results = make(map[string]Pongs)
