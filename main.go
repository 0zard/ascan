package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

func main() {

	var concurrency int
	flag.IntVar(&concurrency, "c", 20, "set the concurrency level")

	var to int
	flag.IntVar(&to, "t", 10000, "timeout (milliseconds)")

	var filename string
	flag.StringVar(&filename, "l", "", "filename location")

	var verbose bool
	flag.BoolVar(&verbose, "v", false, "output errors to stderr")

	flag.Parse()

	// make an actual time.Duration out of the timeout
	timeout := time.Duration(to * 1000000)

	var tr = &http.Transport{
		MaxIdleConns:      30,
		IdleConnTimeout:   time.Second,
		DisableKeepAlives: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: time.Second,
		}).DialContext,
	}

	re := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	client := &http.Client{
		Transport:     tr,
		CheckRedirect: re,
		Timeout:       timeout,
	}

	urls := make(chan string)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)

		go func() {
			for url := range urls {
				if is(client, url) {
					//fmt.Println(url)
					continue
				}
				if verbose {
					fmt.Fprintf(os.Stderr, "failed: %s\n", url)
				}
			}

			wg.Done()
		}()
	}
	if filename != "" {
		file, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			domain := strings.ToLower(scanner.Text())

			// and http
			if !strings.Contains(domain, "http") {
				urls <- "https://" + domain
				urls <- "http://" + domain
			}else{
				urls <- domain
			}
		}
		close(urls)

	} else {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			domain := strings.ToLower(sc.Text())
			
			// and http://
			if !strings.Contains(domain, "http") {
				urls <- "https://" + domain
				urls <- "http://" + domain
			}else {
				urls <- domain
			}
		}
		close(urls)
		if err := sc.Err();err != nil{
			fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
		}
	}
	wg.Wait()
	fmt.Println("over")

}
func is(client *http.Client, urls string) bool {
	//GET
	req, err := http.NewRequest("GET", urls, nil)
	if err != nil {
		return false
	}
	req.Header.Add("Connection", "close")
	req.Close = true

	resp, err := client.Do(req)

	if resp != nil || resp.StatusCode != 200{

		if(scan(resp,urls,"GET")){
			return true
		}
		defer resp.Body.Close()
	}

	//POST
	reqpost, errp := http.NewRequest("POST",urls,nil)
	if errp != nil{
		return false
	}

	reqpost.Header.Add("Connection", "close")
	reqpost.Close = true

	reqpostp,err := client.Do(reqpost)
	if reqpostp != nil{
		if(scan(reqpostp,urls,"POST")){
			return true
		}
	}
	if err!= nil{
		return false
	}

	return true
}
func Gettype(str string) string {
	if strings.Contains(str,"json"){
		return "json"
	}else if(strings.Contains(str,"text/plain")){
		return "text"
	}else if(strings.Contains(str,"text/html")){
		return "html"
	}else if(strings.Contains(str,"application/javascript")){
		return "javascript"
	}else{
		return "unkonw"
	}
}
func findtitle(str string) string {
	matcheds := regexp.MustCompile("<title>[\\s\\S]*?</title>")
	s := matcheds.FindString(str)
	return s

}
func scan(resp *http.Response,url string,method string)  bool{
	if resp.StatusCode == 200{
		body,_ := ioutil.ReadAll(resp.Body)
		str := string(body)
		Type := resp.Header.Get("Content-Type")
		Type = Gettype(Type)
		switch Type{
		case "html":
			title := findtitle(str)
			fmt.Println(url+" "+method+" "+resp.Status+" "+"HTML"+" "+title)
		default:
			if len(str) > 50{
				fmt.Println(url+" "+method+" "+resp.Status+" "+Type+" "+ str[0:50])
			}else {
				fmt.Println(url+" "+method+" "+resp.Status+" "+Type+" "+ str)
			}

		}
		return true
	}else {
		fmt.Println(url+" "+resp.Status)
		return false
	}
	return false
}
