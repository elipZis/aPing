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
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jedib0t/go-pretty/progress"
	"github.com/jedib0t/go-pretty/table"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// A single entry to "ping"
type Ping struct {
	Method  string            `json:"method"`
	Url     string            `json:"url"`
	Headers map[string]string `json:"headers"`
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

// Methods to exclude
var QueryMethods []string

// The HTTP Client to reuse
var client *http.Client

// A pool of Ping objects to reduce the GC overhead
var pingPool = sync.Pool{
	New: func() interface{} {
		return new(Ping)
	},
}

// Define the possible command line arguments
var (
	inputFlag    = flag.String("input", "", "*The path/url to the Swagger/OpenAPI 3.0 input source")
	basePathFlag = flag.String("base", "", "The base url to query")
	outputFlag   = flag.String("out", "console", "The output format. Options: console, csv, html, md")
	headerFlag   = flag.String("header", "{}", "Pass a custom header as JSON string, e.g. '{\"Authorization\": \"Bearer TOKEN\"}'")
	workerFlag   = flag.Int("worker", 1, "The amount of parallel workers to use")
	timeoutFlag  = flag.Int("timeout", 5, "The timeout in seconds per request")
	loopFlag     = flag.Int("loop", 1, "How often to loop through all calls")
	responseFlag = flag.Bool("response", false, "Include the response body in the output")
	methodsFlag  = flag.String("methods", "[\"GET\",\"POST\"]", "An array of query methods to include, e.g. '[\"GET\", \"POST\"]'")

	basePath string
)

// Logging output
var (
	progressWriter    progress.Writer
	tableWriter       table.Writer
	tableColumnConfig = []table.ColumnConfig{
		{Name: "URL"},
		{Name: "Method", WidthMax: 8},
		{Name: "Elapsed ms"},
		{Name: "Response", WidthMax: 100},
	}
)

// Seed
var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

// String charset to randomly pick from
const RandomStringCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Init some short variable options
func init() {
	flag.StringVar(inputFlag, "i", "", "*The path/url to the Swagger/OpenAPI 3.0 input source")
	flag.StringVar(basePathFlag, "b", "", "The base url to query")
	flag.StringVar(outputFlag, "o", "console", "The output format. Options: console, csv, html, md")
	flag.IntVar(workerFlag, "w", 1, "The amount of parallel workers to use")
	flag.IntVar(timeoutFlag, "t", 5, "The timeout in seconds per request")
	flag.IntVar(loopFlag, "l", 1, "How often to loop through all calls")
	flag.BoolVar(responseFlag, "r", false, "Include the response body in the output")
	flag.StringVar(methodsFlag, "m", "[\"GET\",\"POST\"]", "An array of query methods to include, e.g. '[\"GET\", \"POST\"]'")
}

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
		if swagger.Info != nil {
			log.Println(fmt.Sprintf("Pinging '%s - %s'", swagger.Info.Title, swagger.Info.Description))
		} else {
			log.Println(fmt.Sprintf("Pinging '%s - %s'", *inputFlag, *basePathFlag))
		}

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

		// Create a table writer to log to
		tableWriter = table.NewWriter()
		tableWriter.SetAutoIndex(true)
		tableWriter.AppendHeader(table.Row{"URL", "Method", "Elapsed ms", "Response"})
		tableWriter.SetColumnConfigs(tableColumnConfig)

		// Instantiate a Progress Writer and set up the options
		progressWriter = progress.NewWriter()
		progressWriter.SetAutoStop(true)
		progressWriter.ShowTime(true)
		progressWriter.ShowTracker(true)
		progressWriter.ShowValue(true)
		progressWriter.SetNumTrackersExpected(*loopFlag)
		progressWriter.ShowOverallTracker(*loopFlag > 1)
		progressWriter.SetTrackerLength(pings)
		progressWriter.SetSortBy(progress.SortByPercentDsc)
		progressWriter.SetStyle(progress.StyleBlocks)
		progressWriter.SetTrackerPosition(progress.PositionRight)
		progressWriter.SetUpdateFrequency(time.Millisecond * 100)
		progressWriter.Style().Colors = progress.StyleColorsExample
		progressWriter.Style().Options.PercentFormat = "%4.1f%%"
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
		progressWriter.Stop()

		// If an output file is given, write to it
		if outputFlag != nil && *outputFlag != "" {
			switch strings.ToLower(*outputFlag) {
			case "console":
				log.Println("\n" + tableWriter.Render())
			case "csv":
				err := ioutil.WriteFile("aping.csv", []byte(tableWriter.RenderCSV()), 0644)
				checkError(err)
			case "html":
				err := ioutil.WriteFile("aping.html", []byte(tableWriter.RenderHTML()), 0644)
				checkError(err)
			case "md":
				err := ioutil.WriteFile("aping.md", []byte(tableWriter.RenderMarkdown()), 0644)
				checkError(err)
			}
		} else {
			// Otherwise just print the output
			log.Println("\n" + tableWriter.Render())
		}
		return
	}

	// Print the usage in case of no -file given
	flag.Usage()
}

//
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
	for ping := range pings {
		methodName := strings.ToUpper(ping.Method)
		req, err := http.NewRequest(methodName, ping.Url, nil)
		if err != nil {
			log.Println(err)
		}
		req.Close = true

		// Set headers
		for key, value := range ping.Headers {
			req.Header.Set(key, value)
		}

		// Fire
		start := time.Now().UnixNano()
		response, err := client.Do(req)
		elapsed := getElapsedTimeInMS(start)
		// Any error?
		if err != nil {
			tableWriter.AppendRow(table.Row{ping.Url, methodName, elapsed, fmt.Sprintf("[aPing] The HTTP request failed with error %s", err)})
		} else {
			if *responseFlag {
				data, _ := ioutil.ReadAll(response.Body)
				re := regexp.MustCompile(`\r?\n`)
				bodyData := re.ReplaceAllString(string(data), " ")
				tableWriter.AppendRow(table.Row{ping.Url, methodName, elapsed, bodyData})
				_ = response.Body.Close()
			} else {
				tableWriter.AppendRow(table.Row{ping.Url, methodName, elapsed, "-"})
			}
		}

		// Clear up
		waitGroup.Done()
		progressTracker.Increment(1)
	}
}

// Parse any given base url or check for Swagger servers
func parseBase(swagger *openapi3.Swagger) {
	if basePathFlag == nil || *basePathFlag == "" {
		// Check for servers
		var servers []string
		if swagger.Servers != nil {
			for _, v := range swagger.Servers {
				serverUrl := v.URL
				if v.Variables != nil {
					for key, variable := range v.Variables {
						serverUrl = strings.Replace(serverUrl, "{"+key+"}", variable.Default.(string), 1)
					}
				}
				servers = append(servers, serverUrl)
			}
		}

		//
		if servers != nil && len(servers) > 0 {
			fmt.Println("No base given. Select a server.")
			for k, v := range servers {
				fmt.Println(fmt.Sprintf("[%d] %s", k, v))
			}
			fmt.Print("Pick a server no.: ")

			reader := bufio.NewReader(os.Stdin)
			char, _, err := reader.ReadRune()
			if err != nil {
				log.Fatal(err)
			}

			index, err := strconv.Atoi(string(char))
			if err != nil {
				log.Fatal(err)
			}
			if index >= len(servers) {
				log.Println("Cannot parse the given input. Please pick one of the given options as simple number!")
				parseBase(swagger)
			} else {
				basePath = servers[index]
			}
		}
	} else {
		basePath = *basePathFlag
	}
}

// Parse any given header and override/add it to the global header
func parseHeader() {
	var result map[string]string
	err := json.Unmarshal([]byte(*headerFlag), &result)
	checkError(err)

	for key, value := range result {
		Headers[key] = value
	}
}

// Parse all query methods to includefor calls
func parseQueryMethods() {
	err := json.Unmarshal([]byte(*methodsFlag), &QueryMethods)
	checkError(err)
}

// Create a "pingable" url with parameters
func parseUrl(path string, operation *openapi3.Operation) (string, bool) {
	parsed := true
	for _, v := range operation.Parameters {
		// Required or path parameter, which is always required
		if v.Value.Required || strings.ToLower(v.Value.In) == "path" {
			if v.Value.Schema != nil {
				var randomParameter string

				// Check supported parameter types
				switch v.Value.Schema.Value.Type {
				case "integer":
					min := 0
					max := 100
					if v.Value.Schema.Value.Min != nil {
						min = int(*v.Value.Schema.Value.Min)
					}
					if v.Value.Schema.Value.Max != nil {
						max = int(*v.Value.Schema.Value.Max)
					}
					randomParameter = strconv.Itoa(seededRand.Intn(max-min+1) + min)
				case "string":
					length := 1
					if v.Value.Schema.Value.MinLength > 1 {
						length = int(v.Value.Schema.Value.MinLength)
					}
					if v.Value.Schema.Value.MaxLength != nil {
						length = int(*v.Value.Schema.Value.MaxLength)
					}
					randomParameter = getRandString(length)
				default:
					// Cannot parse at least one parameter => don't ping!
					parsed = false
				}
				//
				if randomParameter != "" {
					path = strings.Replace(path, "{"+v.Value.Name+"}", randomParameter, 1)
				}
			} else {
				parsed = false
			}
		}
	}
	return basePath + path, parsed
}

// Return a random string of the given length
func getRandString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = RandomStringCharset[seededRand.Intn(len(RandomStringCharset))]
	}
	return string(b)
}

// Get the elapsed milliseconds from a given starting point
func getElapsedTimeInMS(start int64) int64 {
	return (time.Now().UnixNano() - start) / int64(time.Millisecond)
}

// isValidUrl tests a string to determine if it is a well-structured url or not.
func isValidUrl(toTest string) (*url.URL, bool) {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return nil, false
	}

	u, err := url.Parse(toTest)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, false
	}

	return u, true
}

// Contains a string in a slice
func contains(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

// If an error pops up, fail
func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
