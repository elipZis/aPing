// Copyright (c) 2020 elipZis
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files
// (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge,
// publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR
// ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH
// THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jedib0t/go-pretty/progress"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// The HTTP Client to reuse
var client *http.Client

// Matching pattern for {parameters} in paths
var regExParameterPattern, _ = regexp.Compile("\\{.+\\}")

//
func main() {
	// Parse the input arguments
	flag.Parse()

	// Parse the input file
	if inputFlag == nil || *inputFlag != "" {
		file, err := ioutil.ReadFile(*inputFlag)
		if err != nil {
			log.Fatal(err)
		}

		// Parse the json
		swaggerOpenApi := SwaggerOpenApi{}
		err = json.Unmarshal([]byte(file), &swaggerOpenApi)
		if err != nil {
			log.Fatal(err)
		}

		//
		if swaggerOpenApi.OpenAPI != "" && strings.HasPrefix(swaggerOpenApi.OpenAPI, "3") {
		} else if swaggerOpenApi.Swagger != "" && strings.HasPrefix(swaggerOpenApi.Swagger, "2") {
		} else {
			log.Fatal("The input file does not define its version as Swagger 2.0 or OpenAPI 3.0!", *inputFlag)
		}

		//
		swaggerLoader := &openapi3.SwaggerLoader{
			IsExternalRefsAllowed: true,
		}
		var swagger *openapi3.Swagger
		if validUrl, isValid := isValidUrl(*inputFlag); isValid {
			swagger, err = swaggerLoader.LoadSwaggerFromURI(validUrl)
		} else {
			swagger, err = swaggerLoader.LoadSwaggerFromFile(*inputFlag)
		}
		if err != nil {
			log.Fatal(err)
		}

		// Parse any given header
		parseHeader()
		// Check for a base path
		parseBase(swagger)
		// Check for methods to include
		parseQueryMethods()

		//
		var title string
		if swagger.Info != nil {
			title = fmt.Sprintf("Pinging '%s - %s'", swagger.Info.Title, swagger.Info.Description)
		} else {
			title = fmt.Sprintf("Pinging '%s - %s'", *inputFlag, *basePathFlag)
		}
		log.Println(title)

		// Create a client with timeout and redirect handler
		client = &http.Client{
			Timeout: time.Second * time.Duration(*timeoutFlag),
			// Pass the headers in case of redirects
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				for key, val := range via[0].Header {
					req.Header[key] = val
				}
				return nil
			},
		}

		// Count all pingable routes for a correct output
		var pings int
		for path, pathItem := range swagger.Paths {
			for method, operation := range pathItem.Operations() {
				// Skip non-given methods
				if _, isIncluded := contains(QueryMethods, method); !isIncluded {
					continue
				}
				// Skip routes with request bodies (not supported)
				if operation.RequestBody != nil && operation.RequestBody.Value.Required {
					continue
				}
				// Skip routes we cannot parse (yet)
				if _, parsed := parseUrl(path, operation); parsed {
					pings++
				}
			}
		}

		// Set up the Progress Writer options
		progressWriter.SetNumTrackersExpected(*loopFlag)
		progressWriter.ShowOverallTracker(*loopFlag > 1)
		progressWriter.SetTrackerLength(pings)
		go progressWriter.Render()

		// Prepare the progress trackers
		progressTrackers := make([]progress.Tracker, *loopFlag)
		for i := 0; i < *loopFlag; i++ {
			progressTrackers[i] = progress.Tracker{Message: fmt.Sprintf("Pinging %d routes (Round %d)", pings, i+1), Total: int64(pings), Units: progress.UnitsDefault}
			progressWriter.AppendTracker(&progressTrackers[i])
		}
		// Start looping
		for i := 0; i < *loopFlag; i++ {
			loop(pings, swagger, &progressTrackers[i])
		}
		// Wait for the progress writer to finish rendering
		for progressWriter.IsRenderInProgress() {
			time.Sleep(time.Millisecond * 100)
		}
		progressWriter.Stop()

		// Flush the results
		flush(title, outputFlag)
		return
	}

	// Print the usage in case of no -file given
	flag.Usage()
}

// Loop once through all paths
func loop(pings int, swagger *openapi3.Swagger, progressTracker *progress.Tracker) {
	// Prepare the channels
	var waitGroup sync.WaitGroup
	jobs := make(chan *Ping, pings)

	// Init some workers
	for worker := 0; worker < *workerFlag; worker++ {
		go ping(jobs, &waitGroup, progressTracker)
	}

	// Give the workers something to do (pingpong)
	var ping *Ping
	for path, pathItem := range swagger.Paths {
		for method, operation := range pathItem.Operations() {
			// Skip excluded methods
			if _, isIncluded := contains(QueryMethods, method); !isIncluded {
				continue
			}
			// Skip routes with request bodies (not supported)
			if operation.RequestBody != nil && operation.RequestBody.Value.Required {
				continue
			}
			// Skip routes we cannot parse (yet)
			if pathUrl, parsed := parseUrl(path, operation); parsed {
				// Get a pool ping to reuse
				ping = pingPool.Get().(*Ping)
				ping.Method = method
				ping.Path = path
				ping.Url = pathUrl
				ping.Headers = Headers
				// Fire
				waitGroup.Add(1)
				jobs <- ping
			}
		}
	}
	// Wait for all calls to finish
	waitGroup.Wait()
}

// Ping the given url with all required headers and information
func ping(pings <-chan *Ping, waitGroup *sync.WaitGroup, progressTracker *progress.Tracker) {
	var pong *Pong
	for ping := range pings {
		// The response pool reset object
		pong = pongPool.Get().(*Pong)
		pong.Ping = *ping
		pong.Response = "-"

		methodName := strings.ToUpper(ping.Method)
		req, err := http.NewRequest(methodName, ping.Url, nil)
		if err != nil {
			pong.Response = fmt.Sprintf("[aPing] The new HTTP request build failed with error: %s", err)
		}
		req.Close = true

		// Set headers
		for key, value := range ping.Headers {
			req.Header.Set(key, value)
		}

		// Fire & calculate elapsed ms
		start := time.Now().UnixNano()
		response, err := client.Do(req)
		elapsed := getElapsedTimeInMS(start)
		pong.Time = elapsed

		// Any error?
		if err != nil {
			pong.Response = fmt.Sprintf("[aPing] The HTTP request failed with error: %s", err)
		} else {
			if *responseFlag {
				data, _ := ioutil.ReadAll(response.Body)
				// Trim all line breaks from the response for better output
				re := regexp.MustCompile(`\r?\n`)
				bodyData := re.ReplaceAllString(string(data), " ")
				// Store response
				pong.Response = bodyData
				_ = response.Body.Close()
			}
		}

		// Collect the pongs
		collectPong(pong)

		// Clear & Count up
		progressTracker.Increment(1)
		waitGroup.Done()
		// Return to the source Neo
		pingPool.Put(ping)
	}
}

// Collect and merge/average all
func collectPong(pong *Pong) {
	p, ok := Results[pong.Ping.Path]
	if !ok {
		p = Pongs{
			Path:   pong.Ping.Path,
			Method: pong.Ping.Method,
		}
	}
	if p.Urls == nil || regExParameterPattern.Match([]byte(pong.Ping.Path)) {
		p.Urls = append(p.Urls, pong.Ping.Url)
		p.Responses = append(p.Responses, pong.Response)
	}
	p.Time += pong.Time
	Results[pong.Ping.Path] = p

	// Return to the source Neo
	pongPool.Put(pong)
}
