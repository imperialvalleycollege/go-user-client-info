package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mssola/user_agent"
	"github.com/namsral/flag"
	"github.com/patrickmn/go-cache"
)

var chttp = http.NewServeMux()
var c *cache.Cache
var externalIPs *cache.Cache

var (
	cacheExpiration         int
	cacheLimit              int
	config                  string
	siteTitle               string
	templateFolder          string
	faviconTheme            string
	port                    int
	showExternalIP          bool
	showRecentVisitsLink    bool
	disableRecentVisitsLink bool
	useAsyncView            bool
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

	flag.IntVar(&cacheExpiration, "cache_expiration", 60, "This is the total number of minutes items will remain in the cache.")
	flag.IntVar(&cacheLimit, "cache_limit", 100, "This is the limit for the number of recent visits that will be maintained in memory.")
	flag.StringVar(&config, "config", "", "Path to your config.conf file.")
	flag.StringVar(&siteTitle, "site_title", "User Client Information Application", "The primary title for the application.")
	flag.StringVar(&templateFolder, "template_folder", "default", "Name of the folder to use for loading the template files.")
	flag.StringVar(&faviconTheme, "favicon_theme", "circle-blue", "Name of the folder to use for loading the favicons.")
	flag.IntVar(&port, "port", 3000, "This is the port the HTTP server will use when started.")
	flag.BoolVar(&showExternalIP, "show_external_ip", true, "Toggle the option to display the external IP address.")
	flag.BoolVar(&showRecentVisitsLink, "show_recent_visits_link", true, "Simply hides the /visits URL from the web interface if it's set to false (but it's still accessible directly).")
	flag.BoolVar(&disableRecentVisitsLink, "disable_recent_visits_link", false, "Completely disables the /visits URL (no longer accessible).")
	flag.BoolVar(&useAsyncView, "use_async_view", true, "The /visits URL will use an asynchronous for loading data onto the page.")
	flag.Parse()

	c = cache.New(time.Duration(cacheExpiration)*time.Minute, 30*time.Second)
	externalIPs = cache.New(time.Duration(cacheExpiration)*time.Minute, 30*time.Second)

	// manually switch the recent visits link flag to false if the page is completely disabled:
	if disableRecentVisitsLink {
		showRecentVisitsLink = false
	}

	// Configuration Options Finish:

	// This approach works better than using http.FileServer currently:
	http.HandleFunc("/assets/css/", serveResource)
	http.HandleFunc("/assets/img/", serveResource)
	http.HandleFunc("/assets/js/", serveResource)
	//http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("public/assets"))))

	// Need to use a mux so we can have two handlers for the root path:
	chttp.Handle("/", http.FileServer(http.Dir("public/assets/img/"+faviconTheme)))

	http.HandleFunc("/", root)
	http.HandleFunc("/data", data)
	http.HandleFunc("/visits", visits)
	http.HandleFunc("/external", external)

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
	Timestamp  string `json:"timestamp"`
	IP         string `json:"ip"`
	ExternalIP string `json:"externalIP"`
	Hostname   string `json:"hostname"`
	Browser    string `json:"browser"`
}

// UserInfoList is a list of UserInfo structs
type UserInfoList []UserInfo

func (slice UserInfoList) Len() int {
	return len(slice)
}

func (slice UserInfoList) Less(i, j int) bool {
	return slice[i].Timestamp < slice[j].Timestamp
}

func (slice UserInfoList) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

// Add UserInfo to in-memory cache.
func insertUserInfo(user UserInfo) {

	if c.ItemCount() > cacheLimit {
		c.Flush()
	}

	c.Set(user.Timestamp, user, cache.DefaultExpiration)
}

func insertExternalIP(timestamp string, ip string) {

	if externalIPs.ItemCount() > cacheLimit {
		externalIPs.Flush()
	}

	if _, found := c.Get(timestamp); found == true {
		externalIPs.Set(timestamp, ip, cache.DefaultExpiration)
	}
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

		ua := user_agent.New(r.Header.Get("User-Agent"))
		browserName, browserVersion := ua.Browser()
		userInfo.Browser = browserName + " " + browserVersion

		t := time.Now()
		userInfo.Timestamp = t.Format(time.RFC3339Nano)

		insertUserInfo(userInfo)

		// Setup the Layout:
		layoutPartial := path.Join("public/templates/"+templateFolder, "index.html")
		clientInfoPartial := path.Join("public/templates/"+templateFolder, "client.html")

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
			Data                 *UserInfo
			Count                int
			PageTitle            string
			ShowExternalIP       bool
			ShowRecentVisitsLink bool
			Theme                string
		}{
			&userInfo,
			c.ItemCount(),
			siteTitle,
			showExternalIP,
			showRecentVisitsLink,
			faviconTheme,
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
	if disableRecentVisitsLink {
		fmt.Println("Disabled Visits Link")
		http.NotFound(w, r)
		return
	}
	// The logic for outputting for our in-memory database (with recent request info) should go in here:
	// Setup the Layout:
	async := r.FormValue("async")
	visitsInfoPartial := path.Join("public/templates/"+templateFolder, "visits.html")
	if async == "1" || useAsyncView {
		visitsInfoPartial = path.Join("public/templates/"+templateFolder, "visits_async.html")
	}
	layoutPartial := path.Join("public/templates/"+templateFolder, "index.html")

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
		ExternalIPs    map[string]cache.Item
		PageTitle      string
		ShowExternalIP bool
		Theme          string
	}{
		dataItems,
		c.ItemCount(),
		externalIPs.Items(),
		"Tech Information - " + siteTitle,
		showExternalIP,
		faviconTheme,
	}

	err = templates.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func data(w http.ResponseWriter, r *http.Request) {
	if disableRecentVisitsLink {
		fmt.Println("Disabled Visits Link, so /data endpoint is unavailable")
		http.NotFound(w, r)
		return
	}

	//m := make(map[string]interface{})
	s := make(UserInfoList, c.ItemCount())

	i := 0
	for key, value := range c.Items() {
		//m[key] = value.Object

		var userInfo UserInfo
		userInfo = value.Object.(UserInfo)

		if showExternalIP {
			obj, ok := externalIPs.Get(key)
			if ok {
				userInfo.ExternalIP = obj.(string)
			}
		}

		s[i] = userInfo
		i++
	}

	sort.Sort(sort.Reverse(s))
	callback := r.FormValue("callback")

	// ...

	jsonBytes, err := json.Marshal(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	if callback != "" {
		jsonStr := callback + "(" + string(jsonBytes) + ")"
		jsonBytes = []byte(jsonStr)
		w.Header().Set("Content-Type", "application/javascript")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}

	w.Write(jsonBytes)

	// jData, err := json.Marshal(s)
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// }
	// w.Header().Set("Content-Type", "application/json")
	// w.Write(jData)
}
func external(w http.ResponseWriter, r *http.Request) {
	timestamp := r.URL.Query().Get("timestamp")
	ip := r.URL.Query().Get("ip")

	insertExternalIP(timestamp, ip)
}

func validIP4(ipAddress string) bool {
	ipAddress = strings.Trim(ipAddress, " ")

	re, _ := regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	if re.MatchString(ipAddress) {
		return true
	}
	return false
}
