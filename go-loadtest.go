package main

// @author Robin Verlangen
// Web application loadtester written in Go

import (
    "flag"
    "log"
    "strings"
    "strconv"
    "time"
    "runtime"
    "net"
    "net/http"
    "math/rand"
    "sync"
)

// Variables
var urlsIn string
var concurrency int
var requests int
var urls map[string]int
var urlQueue chan string
var finished chan bool
var timeoutTime int
var waitToFinish bool

// Counters
var startCounter int = int(0)
var startCounterMux sync.Mutex
var doneCounter int = int(0)
var doneCounterMux sync.Mutex

// Init settings
func init() {
    flag.StringVar(&urlsIn, "urls", "", "Urls to benchmark")
    flag.IntVar(&concurrency, "c", 50, "Concurrency")
    flag.IntVar(&requests, "n", 10000, "Amount of requests")
    flag.IntVar(&timeoutTime, "t", 10, "Timeout in seconds")
    flag.Parse()
}

// Requester
func requester() {
    go func() {
        // Client
        transport := &http.Transport{
            Dial: func(netw, addr string) (net.Conn, error) {
                    deadline := time.Now().Add(time.Duration(timeoutTime) * time.Second)
                    c, err := net.DialTimeout(netw, addr, time.Second)
                    if err != nil {
                            return nil, err
                    }
                    c.SetDeadline(deadline)
                    return c, nil
            }}
        httpclient := &http.Client{Transport: transport}

        for {
            var url string
            url = <- urlQueue

            // Prepare url
            url = parseUrl(url)

            // Request
            //log.Println(url)
            resp, err := httpclient.Get(url)
            if err != nil {
                // @todo Error count
            } else {
                // @todo Succes count
                defer resp.Body.Close()
            }

            // Done?
            if waitToFinish && len(urlQueue) == 0 {
                finished <- true
            }
        }
    }()
}

// Parse url
func parseUrl(url string) string {
    url = strings.Replace(url, "{RAND_INT}", strconv.Itoa(rand.Int()), -1)
    return url
}

// Url producer
func urlProducer() {
    go func() {
        for {
            // @todo Support weight
            for url,_ := range urls {
                urlQueue <- url
                startCounter++
            }
            // Stop when we have enough populated
            if startCounter >= requests {
                waitToFinish = true
                log.Printf("%d request are enqueued\n", requests)
                break
            }
        }
    }()
}

// Main
func main() {
    // Queue
    urlQueue = make(chan string, requests)
    finished = make(chan bool, 1)
    waitToFinish = false

    // Use all cores
    runtime.GOMAXPROCS(256)

    // Parse conf
    urls = make(map[string]int)
    splits := strings.Split(urlsIn, ";")
    for _,element := range splits {
        elmSplit := strings.SplitN(element, ":", 2)
        weight, _ := strconv.Atoi(elmSplit[0])
        urls[elmSplit[1]] = weight
    }

    // Filling queue
    urlProducer()

    // Start requesters
    for i := 0; i < concurrency; i++ {
        requester()
    }

    // Wait
    <-finished
    log.Println("Done")
}