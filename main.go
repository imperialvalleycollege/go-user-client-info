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

	"github.com/patrickmn/go-cache"
	"github.com/namsral/flag"
	"github.com/rdegges/go-ipify"
)

var chttp = http.NewServeMux()
var c = cache.New(60*time.Minute, 30*time.Second)

var (
	config         string
	siteTitle      string
	faviconTheme   string
	port           int
	showExternalIP bool

	//userInfoIDs []int
	//userInfoMap map[int]UserInfo
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

	flag.StringVar(&config, "config", "", "Path to your config.conf file.")
	flag.StringVar(&siteTitle, "site_title", "User Client Information Application", "The primary title for the application.")
	flag.StringVar(&faviconTheme, "favicon_theme", "circle-green", "Name of the folder to use for loading the favicons.")
	flag.IntVar(&port, "port", 3000, "This is the port the HTTP server will use when started.")
	flag.BoolVar(&showExternalIP, "show_external_ip", true, "Toggle the option to display the external IP address.")
	flag.Parse()

	// Configuration Options Finish:

	// Initialize UserInfoMap and UserInfoIDs

	//userInfoIDs = make([]int, 0)
	//userInfoMap = make(map[int]UserInfo)

	//http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("public/assets"))))

	// This approach works better than using http.FileServer currently:
	http.HandleFunc("/assets/css/", serveResource)
	http.HandleFunc("/assets/img/", serveResource)
	http.HandleFunc("/assets/js/", serveResource)

	// Need to use a mux so we can have two handlers for the root path:
	chttp.Handle("/", http.FileServer(http.Dir("public/assets/img/"+faviconTheme)))

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

// Add UserInfo to memory.

func insertUserInfo(user UserInfo) {
	//index := len(userInfoIDs)
	//userInfoIDs = append(userInfoIDs, index)
	//userInfoMap[index] = user
	t := time.Now()
	c.Set(t.Format(time.RFC3339Nano), user, cache.DefaultExpiration)
}

// Route functions.
func root(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Reached the root function")
	fmt.Println(r.URL.Path)
	regex := regexp.MustCompile("/([^/]*[^/]*)$")
	matches := regex.FindStringSubmatch(r.URL.Path)

	fmt.Println(matches)

	if r.URL.Path != "/" && len(matches) > 0 {
		fmt.Println("Hitting multiplexer...")

		// First check to see if the provided URL Path
		// matches the name of an actual file in the public
		// directory:
		if info, err := os.Stat("public" + r.URL.Path); err == nil {

			if info.IsDir() {
				fmt.Println("Reached that error")
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

		insertUserInfo(userInfo)

		// Setup the Layout:

		layoutPartial := path.Join("public/templates/default", "mincss.html")
		clientInfoPartial := path.Join("public/templates/default", "mincss_client_info.html")

		// Return a 404 if the template doesn't exist
		info, err := os.Stat(clientInfoPartial)

		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("Reached this error")
				http.NotFound(w, r)
				return
			}
		}

		// Return a 404 if the request is for a directory
		if info.IsDir() {
			fmt.Println("Reached that error")
			http.NotFound(w, r)
			return
		}

		templates, err := template.ParseFiles(layoutPartial, clientInfoPartial)
		if err != nil {
			fmt.Println(err)
			http.Error(w, "500 Internal Server Error", 500)
			return
		}

		data := struct {
			Data  *UserInfo
			SiteTitle string
		}{
			&userInfo,
			siteTitle,
		}

		fmt.Println(userInfo)
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

func tech(w http.ResponseWriter, r *http.Request) {
	// The logic for outputting for our in-memory database (with recent request info) should go in here:
	// Setup the Layout:

	layoutPartial := path.Join("public/templates/default", "mincss.html")
	clientInfoPartial := path.Join("public/templates/default", "mincss_tech_info.html")

	// Return a 404 if the template doesn't exist
	info, err := os.Stat(clientInfoPartial)

	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Reached this error")
			http.NotFound(w, r)
			return
		}
	}

	// Return a 404 if the request is for a directory
	if info.IsDir() {
		fmt.Println("Reached that error")
		http.NotFound(w, r)
		return
	}

	templates := template.New("tech_view")
	templates.Funcs(funcMap)

	templates, err = templates.ParseFiles(layoutPartial, clientInfoPartial)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "500 Internal Server Error", 500)
		return
	}


	data := struct {
		Data map[string]cache.Item
		SiteTitle string
	}{
		c.Items(),
		"Tech Information",
	}
	//fmt.Println(userInfo)
	err = templates.ExecuteTemplate(w, "layout", data)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// UserInfo holds the data that will be displayed onto the page.
type UserInfo struct {
	IP         string
	Hostname   string
	ExternalIP string
}
