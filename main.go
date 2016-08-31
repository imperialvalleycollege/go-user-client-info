package main

import (
	"bufio"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mssola/user_agent"
	"github.com/namsral/flag"
	"github.com/patrickmn/go-cache"
	"github.com/rdegges/go-ipify"
)

var chttp = http.NewServeMux()
var c = cache.New(60*time.Minute, 30*time.Second)

var (
	cacheLimit     int
	config         string
	siteTitle      string
	faviconTheme   string
	port           int
	showExternalIP bool
)

func main() {

	// Configuration Options Start:

	// Forces the config.conf to be processed:
	// If the parameter is not passed in from the CLI
	// but the file exists on the filesystem.
	if NotPassedConfig(os.Args[1:]) {
		if _, err := os.Stat("config.conf"); err == nil {
			fmt.Println("config.conf file exists, will go ahead and use it...")
			os.Args = append(os.Args, "-config=config.conf")
		}
	}

	flag.IntVar(&cacheLimit, "cache_limit", 100, "This is the limit for the number of recent visits that will be maintained in memory.")
	flag.StringVar(&config, "config", "", "Path to your config.conf file.")
	flag.StringVar(&siteTitle, "site_title", "User Client Information Application", "The primary title for the application.")
	flag.StringVar(&faviconTheme, "favicon_theme", "circle-blue", "Name of the folder to use for loading the favicons.")
	flag.IntVar(&port, "port", 3000, "This is the port the HTTP server will use when started.")
	flag.BoolVar(&showExternalIP, "show_external_ip", true, "Toggle the option to display the external IP address.")
	flag.Parse()

	// Configuration Options Finish:

	// This approach works better than using http.FileServer currently:
	http.HandleFunc("/assets/css/", serveResource)
	http.HandleFunc("/assets/img/", serveResource)
	http.HandleFunc("/assets/js/", serveResource)
	//http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("public/assets"))))

	// Need to use a mux so we can have two handlers for the root path:
	chttp.Handle("/", http.FileServer(http.Dir("public/assets/img/"+faviconTheme)))

	http.HandleFunc("/", root)
	http.HandleFunc("/visits", visits)

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

// Using this instead of the regular file server due to mimetype issues:
func serveResource(w http.ResponseWriter, req *http.Request) {
	path := "public" + req.URL.Path
	var contentType string
	if strings.HasSuffix(path, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(path, ".jpg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(path, ".jpeg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(path, ".gif") {
		contentType = "image/gif"
	} else if strings.HasSuffix(path, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(path, ".js") {
		contentType = "application/javascript"
	} else {
		contentType = "text/plain"
	}

	f, err := os.Open(path)

	if err == nil {
		defer f.Close()
		w.Header().Add("Content-Type", contentType)

		br := bufio.NewReader(f)
		br.WriteTo(w)
	} else {
		w.WriteHeader(404)
	}
}

// UserInfo holds the data that will be displayed onto the page.
type UserInfo struct {
	IP         string
	Hostname   string
	ExternalIP string
	Browser    string
}

// Add UserInfo to in-memory cache.
func insertUserInfo(user UserInfo) {

	if c.ItemCount() > cacheLimit {
		c.Flush()
	}

	t := time.Now()
	c.Set(t.Format(time.RFC3339Nano), user, cache.DefaultExpiration)
}

// Route functions.
func root(w http.ResponseWriter, r *http.Request) {
	regex := regexp.MustCompile("/([^/]*[^/]*)$")
	matches := regex.FindStringSubmatch(r.URL.Path)

	if r.URL.Path != "/" && len(matches) > 0 {
		// First check to see if the provided URL Path
		// matches the name of an actual file in the public
		// directory:
		if info, err := os.Stat("public" + r.URL.Path); err == nil {

			if info.IsDir() {
				http.NotFound(w, r)
				return
			}
			http.ServeFile(w, r, "public"+r.URL.Path)
		} else if matches[0] == r.URL.Path {
			chttp.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}

	} else {

		// Collecting our data variables:
		strEntered := r.RemoteAddr
		ipAddr, _, _ := net.SplitHostPort(strEntered)

		forwardedIPs := r.Header.Get("x-forwarded-for")
		// Assuming format is as expected
		ips := strings.Split(forwardedIPs, ", ")
		if len(ips) >= 1 {
			firstIP := ips[0]
			if firstIP != "" && validIP4(firstIP) {
				if ipAddr != firstIP {
					ipAddr = firstIP
				}
			}
		}

		var userInfo UserInfo
		userInfo.IP = ipAddr
		hostnames, _ := net.LookupAddr(ipAddr)

		if len(hostnames) >= 1 {
			userInfo.Hostname = strings.TrimRight(hostnames[0], ".")
		}

		ip, err := ipify.GetIp()
		if err != nil {
			fmt.Println("Couldn't get my IP address:", err)
		} else {
			if showExternalIP {
				userInfo.ExternalIP = ip
			}
		}

		ua := user_agent.New(r.Header.Get("User-Agent"))
		browserName, browserVersion := ua.Browser()
		userInfo.Browser = browserName + " " + browserVersion

		insertUserInfo(userInfo)

		// Setup the Layout:
		layoutPartial := path.Join("public/templates/default", "pure_landing.html")
		clientInfoPartial := path.Join("public/templates/default", "pure_client_info.html")

		// Return a 404 if the template doesn't exist
		info, err := os.Stat(clientInfoPartial)

		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
		}

		// Return a 404 if the request is for a directory
		if info.IsDir() {
			http.NotFound(w, r)
			return
		}

		templates := template.New("client_view")
		templates.Funcs(funcMap)

		templates, err = templates.ParseFiles(layoutPartial, clientInfoPartial)
		if err != nil {
			fmt.Println(err)
			http.Error(w, "500 Internal Server Error", 500)
			return
		}

		data := struct {
			Data           *UserInfo
			Count          int
			PageTitle      string
			ShowExternalIP bool
		}{
			&userInfo,
			c.ItemCount(),
			siteTitle,
			showExternalIP,
		}

		err = templates.ExecuteTemplate(w, "layout", data)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

}

func dateFormat(layout string, d string) string {
	t, _ := time.Parse(time.RFC3339Nano, d)
	var formattedDate string

	formattedDate = t.Format(layout)

	return formattedDate
}

var funcMap = template.FuncMap{
	"dateFormat": dateFormat,
}

func visits(w http.ResponseWriter, r *http.Request) {
	// The logic for outputting for our in-memory database (with recent request info) should go in here:
	// Setup the Layout:

	layoutPartial := path.Join("public/templates/default", "pure_landing.html")
	visitsInfoPartial := path.Join("public/templates/default", "pure_visits_info.html")

	// Return a 404 if the template doesn't exist
	info, err := os.Stat(visitsInfoPartial)

	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
	}

	// Return a 404 if the request is for a directory
	if info.IsDir() {
		http.NotFound(w, r)
		return
	}

	templates := template.New("tech_view")
	templates.Funcs(funcMap)

	templates, err = templates.ParseFiles(layoutPartial, visitsInfoPartial)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "500 Internal Server Error", 500)
		return
	}

	dataItems := c.Items()

	data := struct {
		Data           map[string]cache.Item
		Count          int
		PageTitle      string
		ShowExternalIP bool
	}{
		dataItems,
		c.ItemCount(),
		"Tech Information - " + siteTitle,
		showExternalIP,
	}

	err = templates.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func validIP4(ipAddress string) bool {
	ipAddress = strings.Trim(ipAddress, " ")

	re, _ := regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	if re.MatchString(ipAddress) {
		return true
	}
	return false
}
