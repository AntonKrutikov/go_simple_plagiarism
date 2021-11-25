package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/k3a/html2text"
)

type Result struct {
	Search string `json:"searchRequest"`
	URL    string `json:"url"`
	Before string `json:"before"`
	Found  string `json:"foundText"`
	After  string `json:"after"`
}

type Config struct {
	URL           string
	Search        string
	CountBefore   int
	CountAfter    int
	FuzzyDistance int
	ProxyURL      string
	ProxyAPIKEY   string
}

type customError struct {
	Error      string `json:"error"`
	InnerError string `json:"innerError,omitempty"`
}

func IsUrl(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func responseJson(w http.ResponseWriter, code int, response interface{}) {
	w.WriteHeader(code)
	w.Header().Add("Content-Type", "application/json")
	body, _ := json.Marshal(response)
	w.Write(body)
}

func FuzzySearchHTML(targetStr string, html string, config *Config) (*Result, error) {
	if config == nil {
		config = &Config{}
	}

	//Transform html to plain with "github.com/k3a/html2text"
	text := html2text.HTML2Text(html)

	//Remove newlines
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.ReplaceAll(text, "\n", " ")

	//Create basic Result object
	result := &Result{
		Search: targetStr,
	}

	//Try to find string like targetStr
	match, start, end := match(targetStr, text, config.FuzzyDistance)

	if !match {
		return result, nil //return result with empty Found string (latter check this and provide api "string not found" response)
	}

	//Get founded string
	result.Found = text[start:end]

	//Create regexp for searching surround context of founded string
	//variant that can break words around search (example strings like "Skill:Brand" can produce ":Brand")
	//(?m)((?:[^\s+]+(?:\s+)){0,%d})%s(?:\s+)?((?:[^\s+]+(?:\s+)?){0,%d})
	//variant that return first word only after space char
	//(?m)((?:[^\s+]+(?:\s+)){0,%d})(?:[^\s]+)?%s(?:[^\s]+)?(?:\s+)?((?:[^\s+]+(?:\s+)?){0,%d})
	pattern := fmt.Sprintf(`(?m)((?:[^\s+]+(?:\s+)){0,%d})(?:[^\s]+)?%s(?:[^\s]+)?(?:\s+)?((?:[^\s+]+(?:\s+)?){0,%d})`, config.CountBefore, regexp.QuoteMeta(result.Found), config.CountAfter)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return result, err
	}

	reMatch := re.FindStringSubmatch(text)
	for k, v := range reMatch {
		if k == 1 {
			result.Before = strings.TrimSpace(v)
		}
		if k == 2 {
			result.After = strings.TrimSpace(v)
		}
	}

	return result, nil

}

func searchHandler(w http.ResponseWriter, r *http.Request) {

	config := Config{
		FuzzyDistance: 20,
		CountBefore:   0,
		CountAfter:    0,
		ProxyURL:      "http://api.scraperapi.com/",
		ProxyAPIKEY:   "SECRETKEY",
	}

	r.ParseForm()

	config.URL = r.Form.Get("url")       //REQUIRED
	config.Search = r.Form.Get("search") //REQUIRED
	//Optional: provide fuzzy distance (max count between matched symbols)
	if r.Form.Get("fuzzy_distance") != "" {
		fuzzyDistance, err := strconv.Atoi(r.Form.Get("fuzzy_distance"))
		if err == nil && fuzzyDistance > 0 {
			config.FuzzyDistance = fuzzyDistance
		}
	}
	//Optional: count of words before result
	if r.Form.Get("count_before") != "" {
		countBefore, err := strconv.Atoi(r.Form.Get("count_before"))
		if err == nil {
			config.CountBefore = countBefore
		}
	}
	//Optional: count of words after result
	if r.Form.Get("count_after") != "" {
		countAfter, err := strconv.Atoi(r.Form.Get("count_after"))
		if err == nil {
			config.CountAfter = countAfter
		}
	}

	//Check required params
	if config.URL == "" || config.Search == "" {
		responseJson(w, 400, customError{Error: "Missing required parameters"})
		return
	}

	//URL format scheme+host
	if !IsUrl(config.URL) {
		responseJson(w, 400, customError{Error: "Wrong URL format"})
		return
	}

	//Make request to URL with fallback to proxy
	resp, err := http.Get(config.URL)
	if err != nil || resp.StatusCode != 200 {
		//Fallback proxy api
		proxyURL := fmt.Sprintf("%s?api_key=%s&url=%s", config.ProxyURL, config.ProxyAPIKEY, config.URL)
		resp, err = http.Get(proxyURL)
		if err != nil || resp.StatusCode != 200 {
			responseJson(w, 404, customError{Error: "Not found", InnerError: err.Error()})
			return
		}
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		responseJson(w, 500, customError{Error: "Request error", InnerError: err.Error()})
		return
	}

	result, err := FuzzySearchHTML(config.Search, string(body), &config)
	if err != nil {
		responseJson(w, 500, customError{Error: "Error in parsing response", InnerError: err.Error()})
		return
	}
	result.URL = config.URL

	responseJson(w, 200, result)
}

func match(source, target string, fuzzyDistance int) (bool, int, int) {
	source = strings.ToLower(source)
	target = strings.ToLower(target)

	lenDiff := len(target) - len(source)

	if lenDiff < 0 {
		return false, 0, 0
	}

	if lenDiff == 0 && source == target {
		return true, 0, len(source)
	}

	first := 0
	last := 0
	shift := 0
Loop:
	for {
		first = 0
	Outer:
		for _, r1 := range source {
			if r1 == ' ' {
				continue
			}
			skipped := 0
			for i, r2 := range target {
				shift += utf8.RuneLen(r2)

				if r1 == r2 {
					if first == 0 {
						first = shift - utf8.RuneLen(r2)
					}
					target = target[i+utf8.RuneLen(r2):]
					continue Outer
				}

				skipped++
				if first != 0 && skipped == fuzzyDistance {
					target = target[i+utf8.RuneLen(r2):]
					continue Loop
				}

			}
			return false, 0, 0
		}
		last = shift
		break
	}

	return true, first, last
}

func main() {
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		page := `<!DOCTYPE html><html><body>
		<form action="/api/search" method="POST">
		<input type="text" name="url" placeholder="url" size=100><br>
		<input type="text" name="search" placeholder="string" size=100><br>
		<input type="number" name="count_before" placeholder="count_before" size=100><br>
		<input type="number" name="count_after" placeholder="count_after" size=100><br>
		<input type="number" name="fuzzy_distance" placeholder="fuzzy_distance" size=100><br>

		<input type="submit" value="Search">
		</form>
		</body></html>`

		w.Write([]byte(page))
	})
	http.HandleFunc("/api/search", searchHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
