package main

import (
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/namsral/flag"
)

var chttp = http.NewServeMux()

func main() {

	// Configuration Options Start:
	var (
		config        string
		favicon_theme string
		port          int
	)

	// Forces the config.conf to be processed:
	// If the parameter is not passed in from the CLI
	// but the file exists on the filesystem.
	if NotPassedConfig(os.Args[1:]) {
		if _, err := os.Stat("config.conf"); err == nil {
			fmt.Println("config.conf file exists, will go ahead and use it...")
			os.Args = append(os.Args, "-config=config.conf")
		}
	}

	flag.StringVar(&config, "config", "", "Path to your config.conf file.")
	flag.StringVar(&favicon_theme, "favicon_theme", "circle-green", "Name of the folder to use for loading the favicons.")
	flag.IntVar(&port, "port", 3000, "This is the port the HTTP server will use when started.")
	flag.Parse()

	// Configuration Options Finish:

	http.Handle("/assets/", http.StripPrefix("/public/", http.FileServer(http.Dir("/public/assets/"))))

	chttp.Handle("/", http.FileServer(http.Dir("public/assets/img/"+favicon_theme)))
	// 	fs := http.FileServer(http.Dir("public"))
	// 	http.Handle("/public/", http.StripPrefix("/public/", fs))
	http.HandleFunc("/", root)
	http.HandleFunc("/tech", tech)

	portString := ":" + strconv.Itoa(port)
	fmt.Printf("Listening on port %d...\n", port)

	err := http.ListenAndServe(portString, nil)
	if err != nil {
		log.Fatal("ListenAndServe", err)
		return
	}
}

// NotPassedConfig Check
func NotPassedConfig(args []string) bool {
	for _, val := range args {
		if strings.Contains(val, "config") {
			return false
		}
	}

	return true
}

// Route functions.
func root(w http.ResponseWriter, r *http.Request) {

	regex := regexp.MustCompile("/([^/]*\\.[^/]*)$")
	matches := regex.FindStringSubmatch(r.URL.Path)

	fmt.Println(matches)

	if len(matches) > 0 {
		chttp.ServeHTTP(w, r)
	}

	strEntered := r.RemoteAddr
	ipAddr, _, _ := net.SplitHostPort(strEntered)

	var userInfo UserInfo
	userInfo.IP = ipAddr
	hostnames, _ := net.LookupAddr(ipAddr)

	if len(hostnames) >= 1 {
		userInfo.Hostname = strings.TrimRight(hostnames[0], ".")
	}

	err := rootTemplate.Execute(w, userInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func tech(w http.ResponseWriter, r *http.Request) {
	// The logic for outputting for our in-memory database (with recent request info) should go in here:
}

// UserInfo holds the data that will be displayed onto the page.
type UserInfo struct {
	IP       string
	Hostname string
}

// Templates.
var rootTemplate = template.Must(template.New("root").Parse(rootTemplateHTML))

// Page templates.
const rootTemplateHTML = `
<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<link rel="stylesheet" href="css/upper.css">
	<link rel="apple-touch-icon" sizes="57x57" href="/apple-icon-57x57.png">
	<link rel="apple-touch-icon" sizes="60x60" href="/apple-icon-60x60.png">
	<link rel="apple-touch-icon" sizes="72x72" href="/apple-icon-72x72.png">
	<link rel="apple-touch-icon" sizes="76x76" href="/apple-icon-76x76.png">
	<link rel="apple-touch-icon" sizes="114x114" href="/apple-icon-114x114.png">
	<link rel="apple-touch-icon" sizes="120x120" href="/apple-icon-120x120.png">
	<link rel="apple-touch-icon" sizes="144x144" href="/apple-icon-144x144.png">
	<link rel="apple-touch-icon" sizes="152x152" href="/apple-icon-152x152.png">
	<link rel="apple-touch-icon" sizes="180x180" href="/apple-icon-180x180.png">
	<link rel="icon" type="image/png" sizes="192x192"  href="/android-icon-192x192.png">
	<link rel="icon" type="image/png" sizes="32x32" href="/favicon-32x32.png">
	<link rel="icon" type="image/png" sizes="96x96" href="/favicon-96x96.png">
	<link rel="icon" type="image/png" sizes="16x16" href="/favicon-16x16.png">
	<link rel="manifest" href="/manifest.json">
	<meta name="msapplication-TileColor" content="#ffffff">
	<meta name="msapplication-TileImage" content="/ms-icon-144x144.png">
	<meta name="theme-color" content="#ffffff">
	<title>User Client Information</title>
</head>
<body>
	<h1>Client Information</h1>
	<p>The IP address and Host:</p>
	<pre>{{html .IP}}</pre>
  <pre>{{html .Hostname}}</pre>
</body>
</html>
`
