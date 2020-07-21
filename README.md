# aPing [![GitHub license](https://img.shields.io/github/license/elipzis/aPing.svg)](https://github.com/elipzis/aping/blob/master/LICENSE.md) [![GitHub (pre-)release](https://img.shields.io/badge/release-0.3.0-yellow.svg)](https://github.com/elipzis/aping/releases/tag/0.1.0) [![Donate](https://img.shields.io/badge/Donate-PayPal-green.svg)](https://www.paypal.me/insanitydesign)
A simple API Ping tool to feed an OpenAPI 3.0 description file and call all endpoints

## Features
* Read [OpenAPI 3.0][2] api definition files and call all endpoints
* Ping all endpoints in parallel workers and/or over several loops
* Pass custom headers, e.g. `Authorization`
* Create random `integer` and `string` parameters for urls
* Track the time and response body per request
* Output the results to console, CSV, HTML, JSON or Markdown

## Latest Versions
* 0.4.0
  * Added regular expression filter option for paths
  * Added response time milliseconds threshold
* 0.3.0
  * Added average ms calculation for multiple loops
  * Added Bootstrap HTML template with sortable Table
  * Added JSON output
* 0.2.0
  * Added an option to configure included query methods
  
Download the latest [release here][3].

## Usage
For a quick start download a [release][3], change into the directory and execute the binary with your options, e.g.:
```shell script
./aping -input="calls.json" -header='{\"Authorization\": \"Bearer eyXYZ\"}' -response -base=http://localhost:8080/api -out=html -l=5 -w=5
```

Example progress output:
```
Pinging 'REST API Documentation - Backend'
Pinging 122 routes (Round 1) ... done! [122 in 11.586s]
Pinging 122 routes (Round 2) ... done! [122 in 23.315s]
Pinging 122 routes (Round 3) ... done! [122 in 37.262s]
Pinging 122 routes (Round 4) ... done! [122 in 48.616s]
Pinging 122 routes (Round 5) ... 27.9% ║█████████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░║ [34 in 1m1.386s]
```

### Options
```shell script
Usage
  -input string
        *The path/url to the Swagger/OpenAPI 3.0 input source
  -base string
        The base url to query
  -header string
        Pass a custom header as JSON string, e.g. '{\"Authorization\": \"Bearer TOKEN\"}' (default "{}")
  -loop int
        How often to loop through all calls (default 1)
  -out string
        The output format. Options: console, csv, html, md, json (default "console")
  -response
        Include the response body in the output
  -timeout int
        The timeout in seconds per request (default 5)
  -worker int
        The amount of parallel workers to use (default 1)
  -methods string
        An array of query methods to include, e.g. '[\"GET\", \"POST\"]' (default "[\"GET\",\"POST\"]")
  -threshold int
        Only collect pings above this response threshold in milliseconds (default -1)
  -filter string
        A regular expression to filter paths. Only matches will be pinged!
```

#### Input
Reference a file input somewhere reachable by your machine. 
References in the [OpenAPI][2] specification can be resolved if absolute relative to the main file.

#### Base
Pass a base url such as `http://localhost:8080/api`.
If non is given the `servers` array of the [OpenAPI][2] specification will be presented to pick a server from.

#### Header
Pass custom headers to send with every request as an escaped JSON string such as `'{\"Authorization\": \"Bearer eyXYZ\"}'`.

The default headers are
```
"Accept":       "*/*"
"Connection":   "Keep-Alive"
"Content-Type": "application/json"
"User-Agent":   "aPing"
```

You can override these options by passing the same key.

#### Worker
How many parallel processes should be spawned to query your endpoints.

*Ensure that your endpoint can handle multiple requests, otherwise multiple workers might run into the timeout.*

#### Output
Define an output format. The output is written to a local `aping.XYZ` file, depending on your choice.

#### Loop
*If `loop > 1` is mixed with `response` all responses are logged, if the path has parameters!*

## Build
[Download and install][5] Golang for your platform.

Clone this repository and build your own version:
```shell script
git clone https://github.com/elipZis/aPing.git
go build -o aping github.com/elipZis/aPing
```

### Compatibility
aPing has been tested under the following conditions
* Windows 10 Professional (64-bit)

## Missing/Upcoming Features
aPing is not fully-fledged (yet). Some functionality is missing and errors may occur.

Known issues are:
* Endpoints having request bodies are not pinged
* Parameters besides `integer` and `string` are not pinged 

## License and Credits
aPing is released under the MIT license by [elipZis][1].

This program uses multiple other libraries. Credits and thanks to all the developers working on these great projects:
* Swagger/OpenAPI 3.0 parser [kin-openapi][6]
* Pretty console printer [go-pretty][7]

## Disclaimer
This source and the whole package comes without a warranty. 
It may or may not harm your computer. Please use with care. 
Any damage cannot be related back to the author. 
The source has been tested on a virtual environment and scanned for viruses and has passed all tests.

  [1]: https://elipZis.com
  [2]: https://swagger.io/specification/
  [3]: https://github.com/elipZis/aPing/releases
  [4]: https://github.com/elipZis/aPing/wiki/Version-History
  [5]: https://golang.org/dl/
  [6]: https://github.com/getkin/kin-openapi
  [7]: https://github.com/jedib0t/go-pretty
