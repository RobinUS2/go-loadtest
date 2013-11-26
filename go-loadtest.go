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
var reportInterval int

// Counters
var startCounter int = int(0)
var doneCounter int = int(0)
var doneCounterMux sync.Mutex
var errorCounter int = int(0)
var errorCounterMux sync.Mutex

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
                // Count error
                errorCounterMux.Lock()
                errorCounter++
                errorCounterMux.Unlock()
            } else {
                defer resp.Body.Close()
            }

            // Count finished
            doneCounterMux.Lock()
            doneCounter++
            if doneCounter % reportInterval == 0 {
                printProgress()
            }
            doneCounterMux.Unlock()

            // Done?
            if waitToFinish && len(urlQueue) == 0 && doneCounter >= startCounter {
                finished <- true
            }
        }
    }()
}

// Progress
func printProgress() {
    var perc float32 = float32(doneCounter) / float32(requests) * float32(100)
    log.Printf("Finished %d requests (%3.1f%%)", doneCounter, perc)
}

// Summary
func printSummary() {
    log.Printf("----- SUMMARY -----")
    log.Printf("Total %d requests", doneCounter)
    log.Printf("Successful %d requests", doneCounter - errorCounter)
    log.Printf("Failed %d requests", errorCounter)
    log.Printf("-------------------")
}

// Parse url
func parseUrl(url string) string {
    url = strings.Replace(url, "{RAND_INT}", strconv.Itoa(rand.Int()), -1)
    return url
}

// Url producer
func urlProducer() {
    go func() {
        // Calculate total
        var totalWeight = 0
        for _,weight := range urls {
            totalWeight += weight
        }
        log.Printf("%d total weight\n", totalWeight)

        // Start populating
        for {
            // Spawn urls with regard to weighting
            for url,weight := range urls {
                var chance = totalWeight / weight
                for i := 0; i < chance; i++ {
                    urlQueue <- url
                    startCounter++

                    // Stop if we have enough
                    if startCounter >= requests {
                        break
                    }
                }
            }
            // Stop when we have enough populated
            if startCounter >= requests {
                waitToFinish = true
                //log.Printf("%d request are enqueued\n", requests)
                break
            }
        }
    }()
}

// Info
func printInfo() {
    log.Println("Starting go-loadtest")
    log.Println("Documentation: https://github.com/RobinUS2/go-loadtest")
    log.Println("-------------------")
}

// Main
func main() {
    // Info
    printInfo()

    // Queue
    urlQueue = make(chan string, requests)
    finished = make(chan bool, 1)
    waitToFinish = false
    reportInterval = requests / 10

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

    // Settings
    log.Printf("Located %d urls\n", len(urls))
    log.Printf("Concurrency is %d client(s)\n", concurrency)
    log.Printf("Amount of requests %d\n", requests)
    log.Printf("Maximum response time %d second(s)\n", timeoutTime)

    // Start
    log.Println("Starting loadtest")
    log.Println("-------------------")

    // Filling queue
    urlProducer()

    // Start requesters
    for i := 0; i < concurrency; i++ {
        requester()
    }

    // Wait
    <-finished
    printProgress()
    printSummary()
    log.Println("Done go-loadtest")
}