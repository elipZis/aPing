package main

import (
	"encoding/json"
	"github.com/jedib0t/go-pretty/table"
	"io/ioutil"
	"log"
	"strings"
	"time"
)

var HtmlTemplate = `
<!doctype html>

<html lang="en">
<head>
  <meta charset="utf-8">

  <title>aPing - Results</title>
  <meta name="description" content="A simple API Ping tool to feed an OpenAPI 3.0 description file and call all endpoints">
  <meta name="author" content="elipZis">
  <meta name="viewport" content="width=device-width, initial-scale=1">

  <link rel="stylesheet" href="https://stackpath.bootstrapcdn.com/bootstrap/4.5.0/css/bootstrap.min.css">

  <style>
	th[role=columnheader]:not(.no-sort) {
		cursor: pointer;
	}
	
	th[role=columnheader]:not(.no-sort):after {
		content: '';
		float: right;
		margin-top: 7px;
		border-width: 0 4px 4px;
		border-style: solid;
		border-color: #404040 transparent;
		visibility: hidden;
		opacity: 0;
		-ms-user-select: none;
		-webkit-user-select: none;
		-moz-user-select: none;
		user-select: none;
	}
	
	th[aria-sort=ascending]:not(.no-sort):after {
		border-bottom: none;
		border-width: 4px 4px 0;
	}
	
	th[aria-sort]:not(.no-sort):after {
		visibility: visible;
		opacity: 0.4;
	}
	
	th[role=columnheader]:not(.no-sort):hover:after {
		visibility: visible;
		opacity: 1;
	}
  </style>
</head>

<body>
  <h2>aPing - Results</h2>
  <h4>{{TITLE}} @ {{DATE}}</h4>

  <div class="container-fluid">
    <div class="row">
      {{TABLE}}
    </div>
  </div>

  <footer class="page-footer font-small blue pt-4">
    <div class="footer-copyright text-center py-3">
      Created with <a href="https://github.com/elipZis/aPing">aPing</a> © 2020 Copyright <a href="https://elipZis.com/">elipZis</a>
    </div>
  </footer>

  <script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/3.5.1/jquery.min.js"></script>
  <script src="https://stackpath.bootstrapcdn.com/bootstrap/4.5.0/js/bootstrap.bundle.min.js"></script>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/tablesort/5.2.1/tablesort.min.js"></script>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/tablesort/5.2.1/sorts/tablesort.number.min.js"></script>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/tablesort/5.2.1/sorts/tablesort.monthname.min.js"></script>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/tablesort/5.2.1/sorts/tablesort.filesize.min.js"></script>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/tablesort/5.2.1/sorts/tablesort.dotsep.min.js"></script>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/tablesort/5.2.1/sorts/tablesort.date.min.js"></script>

  <script>
    new Tablesort(document.getElementsByClassName('aping-table')[0]);
  </script>
</body>
</html>
`

// Result table collector
var (
	tableWriter       table.Writer
	tableColumnConfig = []table.ColumnConfig{
		{Name: "Path"},
		{Name: "URL"},
		{Name: "Method", WidthMax: 8},
		{Name: "Avg. ms"},
		{Name: "Response", WidthMax: 100},
	}
)

// Flush all collected results to the aspired output
func flush(title string, output *string) {
	// Create a table writer to log to
	tableWriter = table.NewWriter()
	tableWriter.SetAutoIndex(true)
	tableWriter.AppendHeader(table.Row{"Path", "URL", "Method", "Avg. ms", "Response"})
	tableWriter.SetColumnConfigs(tableColumnConfig)
	tableWriter.SetHTMLCSSClass("sort table table-striped table-hover table-responsive aping-table")

	// Flush the pongs
	for _, result := range Results {
		tableWriter.AppendRow(table.Row{
			result.Path,
			strings.Join(result.Urls, "\r\n"),
			result.Method,
			result.Time / int64(*loopFlag),
			strings.Join(result.Responses, "\r\n"),
		})
	}

	// If an output file is given, write to it
	if output != nil && *output != "" {
		switch strings.ToLower(*output) {
		case "console":
			log.Println("\n" + tableWriter.Render())
		case "csv":
			err := ioutil.WriteFile("aping.csv", []byte(tableWriter.RenderCSV()), 0644)
			checkFatalError(err)
		case "html":
			html := strings.Replace(HtmlTemplate, "{{TITLE}}", title, 1)
			html = strings.Replace(html, "{{DATE}}", time.Now().Format("2006-01-02 15:04:05"), 1)
			html = strings.Replace(html, "{{TABLE}}", tableWriter.RenderHTML(), 1)
			err := ioutil.WriteFile("aping.html", []byte(html), 0644)
			checkFatalError(err)
		case "md":
			err := ioutil.WriteFile("aping.md", []byte(tableWriter.RenderMarkdown()), 0644)
			checkFatalError(err)
		case "json":
			file, _ := json.MarshalIndent(Results, "", " ")
			err := ioutil.WriteFile("aping.json", file, 0644)
			checkFatalError(err)
		}
	} else {
		// Otherwise just print the output
		log.Println("\n" + tableWriter.Render())
	}
}
