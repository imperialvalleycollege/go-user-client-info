package main

import (
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

func main() {
	// 	fs := http.FileServer(http.Dir("public"))
	// 	http.Handle("/public/", http.StripPrefix("/public/", fs))
	http.HandleFunc("/", root)
	http.HandleFunc("/tech", tech)

	fmt.Println("Listening...")
	err := http.ListenAndServe(GetPort(), nil)
	if err != nil {
		log.Fatal("ListenAndServe", err)
		return
	}
}

func GetPort() string {
	// var port = os.Getenv("PORT")
	var port string
	if len(os.Args) >= 2 {
		port = os.Args[1]
	}
	if port == "" {
		port = "3000"
		fmt.Println("INFO: No PORT environment variable detected, defaulting to " + port)
	}
	return ":" + port
}

// Route functions.
func root(w http.ResponseWriter, r *http.Request) {

	strEntered := r.RemoteAddr
	ipAddr, _, _ := net.SplitHostPort(strEntered)

	var userInfo UserInfo
	userInfo.Ip = ipAddr
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

// Structures.
type UserInfo struct {
	Ip       string
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
	<title>User Client Information</title>
</head>
<body>
	<h1>Client Information</h1>
	<p>The IP address and Host:</p>
	<pre>{{html .Ip}}</pre>
  <pre>{{html .Hostname}}</pre>
</body>
</html>
`
