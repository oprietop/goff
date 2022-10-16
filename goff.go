package main

import (
    "fmt"
    "os"
    "time"
    "io"
    "io/ioutil"
    "log"
    "net/http"
    "net/url"
    "strings"
    "sync"
)

const MAX_CONCURRENT_JOBS = 5

// Log error
func errLog(err error) {
    if err != nil {
        log.Printf("Error: %v", err)
    }
}

// Log error and exit 1
func errFail(err error) {
    if err != nil {
        log.Fatalf("Error: %v", err)
    }
}

// Main struct
type RunTasks struct {
    mutex     sync.Mutex
    client    *http.Client
    waitGroup sync.WaitGroup
    waitChan  chan struct{}
    fileUrls  map[string]string
    loopUrls  map[string]string
}

// Struct generator
func NewRunTasks() *RunTasks {
    tr := &http.Transport{}
    client := &http.Client{Transport: tr}
    return &RunTasks{
        client: client,
        waitChan: make(chan struct{}, MAX_CONCURRENT_JOBS),
        fileUrls: map[string]string{},
        loopUrls: map[string]string{},
    }
}

// HTTP fetch
func (rt *RunTasks) fetch(uri string) (result []byte) {
    req, _ := http.NewRequest("GET", uri, nil)
    res, err := rt.client.Do(req)
    //res, err := http.Get(uri)
    errLog(err)
    defer res.Body.Close()

    body, err := ioutil.ReadAll(res.Body)
    errLog(err)

    return body
}

// HTTP download to disk
func (rt *RunTasks) download(uri string, name string) (err error) {
    //res, err := http.Get(uri)
    req, _ := http.NewRequest("GET", uri, nil)
    res, err := rt.client.Do(req)
    if err != nil  {
        return err
    }
    defer res.Body.Close()

    if res.StatusCode != http.StatusOK {
        log.Println("Non-OK HTTP status:", res.StatusCode, "fetching", name)
        return fmt.Errorf("%i", res.StatusCode)
    }

    if strings.Contains(res.Header["Content-Type"][0], "down") {
        log.Println("Fetching", name)
    }

    // Create the file and return the handle
    out, err := os.Create(name)
    if err != nil  {
        return err
    }
    defer out.Close()

    // Store the payload in the file handle
    _, err = io.Copy(out, res.Body)
    if err != nil  {
        return err
    }

    // Everything went well
    if strings.Contains(res.Header["Content-Type"][0], "down") {
        log.Println("Finished", name)
        return nil
    }

    return fmt.Errorf("%s - %d - %s", name, res.StatusCode, res.Header["Content-Type"][0])
}

// Retrieve the real download URL and filename
func (rt *RunTasks) getDownUri(uri string) (downUri string, file string) {
    //page := rt.fetch("GET", uri, nil)
    page := rt.fetch(uri)
    splitSD := strings.Split(string(page), "\">Start Download")
    if len(splitSD) == 1 {
        return "", ""
    }
    splitHref := strings.Split(splitSD[0], "data-href=\"")
    downUri = splitHref[len(splitHref)-1]
    splitUri := strings.Split(downUri, "/")
    file = splitUri[len(splitUri)-1]

    return downUri, file
}

// Remove finished downloads from urls.txt
func (rt *RunTasks) updateTxt(uri string) (err error) {
    defer rt.mutex.Unlock()
    rt.mutex.Lock()

    delete(rt.fileUrls, uri)

    strUrls := ""
    for u := range rt.fileUrls {
        strUrls = strUrls + u + "\n"
    }

    dataBytes := []byte(strUrls)
    ioutil.WriteFile("urls.txt", dataBytes, 0)

    return nil
}

// Wrapper function to run under goroutines
func (rt *RunTasks) job(uri string) (err error) {

    // Get the real download url and filename
    downUri, file := rt.getDownUri(uri)
    if downUri == "" {
        log.Println("Cound not find file in", uri, "skipping.")
        return nil
    }
    log.Println(uri, downUri, file)
    if _, err := os.Stat(file); err == nil {
        log.Println(file, "exists, skipping.")
    } else {
        // Loop until we got our file
        for {
            err := rt.download(downUri, file)
            //log.Println(downUri, file)
            if err == nil  {
                break
            }
            //log.Println(err)
            time.Sleep(1 * time.Minute)
        }
    }
    // Remove the link from urls.txt
    rt.updateTxt(uri)

    return nil
}

func main() {
    rt := NewRunTasks()
    log.Printf("%+v\n", rt)

    content, err := ioutil.ReadFile("urls.txt")
    if err != nil {
        errFail(err)
    }
    lines := strings.Split(string(content), "\n")

    for _, line := range lines {
        line = strings.ToLower(line)
        line = strings.TrimSpace(line)
        _, err := url.ParseRequestURI(line)
        if err == nil {
            rt.fileUrls[line] = ""
            rt.loopUrls[line] = ""
        }
    }

    for uri, _ := range rt.loopUrls {
        rt.waitGroup.Add(1)
        rt.waitChan <- struct{}{}
        go func(uri string) {
            defer rt.waitGroup.Done()
            rt.job(uri)
            <-rt.waitChan
        }(uri)
    }

    rt.waitGroup.Wait()
}
