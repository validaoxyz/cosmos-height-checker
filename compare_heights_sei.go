package main

import (
  "encoding/json"
  "fmt"
  "io/ioutil"
  "net/http"
  "regexp"
  "sync"
  "strconv"
  "time"
)

type NetInfo struct {
  Peers []struct {
    URL string `json:"url"`
  } `json:"peers"`
}

type Status struct {
  SyncInfo struct {
    LatestBlockHeight string `json:"latest_block_height"`
  } `json:"sync_info"`
}

var initialRPCs = []string{
  "http://localhost:26657",
  "https://sei-rpc.polkachu.com",
  "https://sei-rpc.lavenderfive.com:443",
  "https://sei-rpc.brocha.in",
  "https://rpc-sei.stingray.plus",
  "https://rpc-sei.rhinostake.com",
}

// fetchNetInfo gets list of peers from a an RPC
func fetchNetInfo(url string) ([]string, error) {
    client := &http.Client{
        Timeout: 800 * time.Millisecond,
    }

    resp, err := client.Get(url + "/net_info")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    var info NetInfo
    err = json.Unmarshal(body, &info)
    if err != nil {
        return nil, err
    }

    ips := []string{}
    for _, peer := range info.Peers {
        re := regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
        ip := re.FindString(peer.URL)
        if ip != "" {
            ips = append(ips, ip)
        }
    }

    return ips, nil
}

// fetchBlockHeightWithTimeout gets block height from an rpc (with timeout
func fetchBlockHeightWithTimeout(url string, ch chan<- string) {
  client := &http.Client{
    Timeout: 150 * time.Millisecond,
  }

  resp, err := client.Get(url + "/status")
  if err != nil {
    ch <- ""
    return
  }
  defer resp.Body.Close()

  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    ch <- ""
    return
  }

  var status Status
  if err := json.Unmarshal(body, &status); err != nil {
    ch <- ""
    return
  }

  ch <- status.SyncInfo.LatestBlockHeight
}

// gets the block height from rpc
func fetchBlockHeight(url string) (string, error) {
  resp, err := http.Get(url + "/status")
  if err != nil {
    return "", err
  }
  defer resp.Body.Close()

  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return "", err
  }

  var status Status
  err = json.Unmarshal(body, &status)
  if err != nil {
    return "", err
  }

  return status.SyncInfo.LatestBlockHeight, nil
}

// recursively crawl network
func crawlNetwork(rpc string, knownNodes map[string]bool) {
    ips, err := fetchNetInfo(rpc)
    if err != nil {
        return
    }

    for _, ip := range ips {
        if !knownNodes[ip] {
            knownNodes[ip] = true
            // Immediately crawl this new node before continuing with the others
            crawlNetwork("http://"+ip+":26657", knownNodes)
        }
    }
}

// returns a slice of keys from a map
func getKeysFromMap(m map[string]bool) []string {
  keys := make([]string, 0, len(m))
  for k := range m {
    keys = append(keys, k)
  }
  return keys
}


func main() {
  // var highestBlockHeight string
  // var localBlockHeight string
  var mu sync.Mutex
  status := "GOOD"
  // lastGoodTime := time.Now()

  // HTTP server for the /height health endpoint
  http.HandleFunc("/height", func(w http.ResponseWriter, r *http.Request) {
    mu.Lock()
    defer mu.Unlock()
    // if time.Since(lastGoodTime) > 1*time.Minute {
    //   status = "BAD"
    // }
    fmt.Fprint(w, status)
  })
  go http.ListenAndServe(":8080", nil)

  // goroutine to crawl the network once every hour
  go func() {
    for {
        var allIPs []string
        for _, rpc := range initialRPCs {
            ips, err := fetchNetInfo(rpc)
            if err == nil {
                allIPs = append(allIPs, ips...)
            }
        }

        ch := make(chan string, len(allIPs))
        for _, ip := range allIPs {
            go fetchBlockHeightWithTimeout("http://"+ip+":26657", ch)
        }

        highestBlockHeightInt := 0
        for i := 0; i < len(allIPs); i++ {
            heightStr := <-ch
            heightInt, err := strconv.Atoi(heightStr)
            if err == nil {
                mu.Lock()
                if heightInt > highestBlockHeightInt {
                    highestBlockHeightInt = heightInt
                }
                mu.Unlock()
            }
        }

        // Fetch local block height immediately after fetching from peers (ours last to reduce false positives)
        localHeightStr, err := fetchBlockHeight("http://localhost:26657")
        if err == nil {
            localHeightInt, err := strconv.Atoi(localHeightStr)
            if err == nil {
                mu.Lock()
                if localHeightInt >= highestBlockHeightInt {
                    // lastGoodTime = time.Now()
                    status = "GOOD"  // status for http hander, reset to GOOD here
                }
                if localHeightInt < highestBlockHeightInt {
                    status = "BAD"
                    fmt.Printf("OOPS    Highest: %v Ours: %v \n", highestBlockHeightInt, localHeightInt)
                } else {
                    fmt.Printf("GOOD    Highest: %v Ours: %v \n", highestBlockHeightInt, localHeightInt)
                }
                mu.Unlock()
            }
        }

        time.Sleep(1 * time.Second)
    }
}()


  select {} // hack to keep the main function running and not ending
}
//
