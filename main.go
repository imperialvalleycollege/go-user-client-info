package main

import (
	"fmt"
	"html/template"
	"log"
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
	//strEntered := r.FormValue("str")
	// https://golang.org/pkg/strings/#Split
	strEntered := r.RemoteAddr
	ipAddr := strings.Split(strEntered, ":")
	strUpper := strings.ToUpper(ipAddr[0])
	// 	strUpper := strings.ToUpper(strEntered)
	err := upperTemplate.Execute(w, strUpper)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
	<pre>{{html .}}</pre>
</body>
</html>
`
