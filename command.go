package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jedib0t/go-pretty/progress"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Methods to exclude
var QueryMethods []string

// Define the possible command line arguments
var (
	inputFlag     = flag.String("input", "", "*The path/url to the Swagger/OpenAPI 3.0 input source")
	basePathFlag  = flag.String("base", "", "The base url to query")
	outputFlag    = flag.String("out", "console", "The output format. Options: console, csv, html, md, json")
	headerFlag    = flag.String("header", "{}", "Pass a custom header as JSON string, e.g. '{\"Authorization\": \"Bearer TOKEN\"}'")
	workerFlag    = flag.Int("worker", 1, "The amount of parallel workers to use")
	timeoutFlag   = flag.Int("timeout", 5, "The timeout in seconds per request")
	loopFlag      = flag.Int("loop", 1, "How often to loop through all calls")
	responseFlag  = flag.Bool("response", false, "Include the response body in the output")
	methodsFlag   = flag.String("methods", "[\"GET\",\"POST\"]", "An array of query methods to include, e.g. '[\"GET\", \"POST\"]'")
	filterFlag    = flag.String("filter", "", "A regular expression to filter matching paths. Only will be pinged!")
	thresholdFlag = flag.Int("threshold", -1, "Only collect pings above this response threshold in milliseconds")

	basePath string
)

// RegExp pattern for path filter
var regExPathFilterPattern *regexp.Regexp

// Logging output
var progressWriter = progress.NewWriter()

// Init some short variable options
func init() {
	flag.StringVar(inputFlag, "i", "", "*The path/url to the Swagger/OpenAPI 3.0 input source")
	flag.StringVar(basePathFlag, "b", "", "The base url to query")
	flag.StringVar(outputFlag, "o", "console", "The output format. Options: console, csv, html, md, json")
	flag.IntVar(workerFlag, "w", 1, "The amount of parallel workers to use")
	flag.IntVar(timeoutFlag, "t", 5, "The timeout in seconds per request")
	flag.IntVar(loopFlag, "l", 1, "How often to loop through all calls")
	flag.BoolVar(responseFlag, "r", false, "Include the response body in the output")
	flag.StringVar(methodsFlag, "m", "[\"GET\",\"POST\"]", "An array of query methods to include, e.g. '[\"GET\", \"POST\"]'")
	flag.StringVar(filterFlag, "f", "", "A regular expression to filter matching paths. Only will be pinged!")

	// Pre-set the progress writer
	progressWriter.SetAutoStop(true)
	progressWriter.ShowTime(true)
	progressWriter.ShowTracker(true)
	progressWriter.ShowValue(true)
	progressWriter.SetSortBy(progress.SortByPercentDsc)
	progressWriter.SetStyle(progress.StyleBlocks)
	progressWriter.SetTrackerPosition(progress.PositionRight)
	progressWriter.SetUpdateFrequency(time.Millisecond * 100)
	progressWriter.Style().Colors = progress.StyleColorsExample
	progressWriter.Style().Options.PercentFormat = "%4.1f%%"
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
	checkFatalError(err)

	for key, value := range result {
		Headers[key] = value
	}
}

// Check for a path filter regular expression
func parseFilter() {
	if filterFlag != nil && *filterFlag != "" {
		var err error
		regExPathFilterPattern, err = regexp.Compile(*filterFlag)
		checkFatalError(err)
	}
}

// Parse all query methods to includefor calls
func parseQueryMethods() {
	err := json.Unmarshal([]byte(*methodsFlag), &QueryMethods)
	checkFatalError(err)
}

// Create a "pingable" url with parameters
func parseUrl(path string, operation *openapi3.Operation) (string, bool) {
	// Filter paths, if set
	if regExPathFilterPattern != nil && !regExPathFilterPattern.Match([]byte(path)) {
		return "", false
	}

	//
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
