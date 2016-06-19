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
	fs := http.FileServer(http.Dir("public"))
	http.Handle("/public/", http.StripPrefix("/public/", fs))

	http.HandleFunc("/upper", upper)

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
		port = "4747"
		fmt.Println("INFO: No PORT environment variable detected, defaulting to " + port)
	}
	return ":" + port
}

var upperTemplate = template.Must(template.New("upper").Parse(upperTemplateHTML))

func upper(w http.ResponseWriter, r *http.Request) {

	strEntered := r.RemoteAddr
	strUpper, _, _ := net.SplitHostPort(strEntered)
	// 	ipAddr := strings.Split(strEntered, ":")
	// 	strUpper := strings.ToUpper(ipAddr[0])

	var userInfo UserInfo
	userInfo.Ip = strUpper
	hostnames, _ := net.LookupAddr(strUpper)

	if len(hostnames) >= 1 {
		userInfo.Hostname = strings.TrimRight(hostnames[0], ".")
	}

	err := upperTemplate.Execute(w, userInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ip and hostname for user info struct
// https://www.golang-book.com/books/intro/9
type UserInfo struct {
	Ip       string
	Hostname string
}

const upperTemplateHTML = `
<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<link rel="stylesheet" href="css/upper.css">
	<title>String Upper Results</title>
</head>
<body>
	<h1>String Upper Results</h1>
	<p>The Uppercase of the string that you had entered is:</p>
	<pre>{{html .Ip}}</pre>
  <pre>{{html .Hostname}}</pre>
</body>
</html>
`
