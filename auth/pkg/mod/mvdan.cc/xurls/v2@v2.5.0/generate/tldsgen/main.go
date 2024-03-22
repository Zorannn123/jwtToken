// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"text/template"
)

const path = "tlds.go"

var tldsTmpl = template.Must(template.New("tlds").Parse(`// Generated by tldsgen

package xurls

// TLDs is a sorted list of all public top-level domains.
//
// Sources:{{range $_, $url := .URLs}}
//   - {{$url}}{{end}}
var TLDs = []string{
{{range $_, $tld := .TLDs}}` + "\t`" + `{{$tld}}` + "`" + `,
{{end}}}
`))

func cleanTld(tld string) string {
	tld = strings.ToLower(tld)
	if strings.HasPrefix(tld, "xn--") {
		return ""
	}
	return tld
}

func fetchFromURL(wg *sync.WaitGroup, url, pat string, tldSet map[string]bool) {
	defer wg.Done()
	log.Printf("Fetching %s", url)
	resp, err := http.Get(url)
	if err == nil && resp.StatusCode >= 400 {
		err = errors.New(resp.Status)
	}
	if err != nil {
		panic(fmt.Errorf("%s: %s", url, err))
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	re := regexp.MustCompile(pat)
	for scanner.Scan() {
		line := scanner.Text()
		tld := re.FindString(line)
		tld = cleanTld(tld)
		if tld == "" {
			continue
		}
		tldSet[tld] = true
	}
	if err := scanner.Err(); err != nil {
		panic(fmt.Errorf("%s: %s", url, err))
	}
}

func tldList() ([]string, []string) {
	var urls []string
	var wg sync.WaitGroup
	tldSet := make(map[string]bool)
	fromURL := func(url, pat string) {
		urls = append(urls, url)
		wg.Add(1)
		go fetchFromURL(&wg, url, pat, tldSet)
	}
	fromURL("https://data.iana.org/TLD/tlds-alpha-by-domain.txt", `^[^#]+$`)
	fromURL("https://publicsuffix.org/list/effective_tld_names.dat", `^[^/.]+$`)
	wg.Wait()

	tlds := make([]string, 0, len(tldSet))
	for tld := range tldSet {
		tlds = append(tlds, tld)
	}

	sort.Strings(tlds)
	return tlds, urls
}

func writeTlds(tlds, urls []string) error {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	return tldsTmpl.Execute(f, struct {
		TLDs []string
		URLs []string
	}{
		TLDs: tlds,
		URLs: urls,
	})
}

func main() {
	tlds, urls := tldList()
	log.Printf("Generating %s...", path)
	writeTlds(tlds, urls)
}