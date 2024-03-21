package main

import (
    "container/list"
    "net/http/httputil"
    "net/http"
    "net/url"
    "net"
    "io/ioutil"
)

var servers *list.List

func helloHandler(w http.ResponseWriter, r *http.Request) {
    for e := servers.Front(); e != nil; e = e.Next() {
        server := e.Value.(*url.URL)

        conn, err := net.Dial("tcp", server.Host)
        if err != nil {
            // remove the server from the list
            next := e.Next()
            servers.Remove(e)
            e = next
            continue
        }

        conn.Close()

        proxy := httputil.NewSingleHostReverseProxy(server)
        proxy.ServeHTTP(w, r)

        // Move the server to the back of the list
        servers.MoveToBack(e)
        break
    }

    if servers.Len() == 0 {
        http.Error(w, "No servers available", http.StatusInternalServerError)
        return
    }
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
    serverURL, err := url.Parse(r.URL.Query().Get("url"))
    if err != nil {
        http.Error(w, "Invalid server URL", http.StatusBadRequest)
        return
    }

    servers.PushBack(serverURL)
}

func listServersHandler(w http.ResponseWriter, r *http.Request) {
    for e := servers.Front(); e != nil; e = e.Next() {
        server := e.Value.(*url.URL)
        w.Write([]byte(server.String() + "\n"))
    }
}


func serverStatsHandler(w http.ResponseWriter, r *http.Request) {
    for e := servers.Front(); e != nil; e = e.Next() {
        server := e.Value.(*url.URL)
        statsURL := server.String() + "/stats" // assuming "/stats" endpoint returns CPU and memory usage

        resp, err := http.Get(statsURL)
        if err != nil {
            http.Error(w, "Error getting server stats", http.StatusInternalServerError)
            return
        }
        defer resp.Body.Close()

        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            http.Error(w, "Error reading server stats", http.StatusInternalServerError)
            return
        }

        w.Write([]byte(server.String() + ": " + string(body) + "\n"))
    }
}

func cpuHandler(w http.ResponseWriter, r *http.Request) {
    for e := servers.Front(); e != nil; e = e.Next() {
        server := e.Value.(*url.URL)
        statsURL := server.String() + "/cpu" // assuming "/cpu" endpoint returns CPU usage

        resp, err := http.Get(statsURL)
        if err != nil {
            http.Error(w, "Error getting server stats", http.StatusInternalServerError)
            return
        }
        defer resp.Body.Close()

        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            http.Error(w, "Error reading server stats", http.StatusInternalServerError)
            return
        }

        w.Write([]byte(server.String() + ": " + string(body) + "\n"))
    }
}

func main() {
    servers = list.New()

    http.HandleFunc("/", helloHandler)
    http.HandleFunc("/register", registerHandler)
    http.HandleFunc("/list", listServersHandler)
    http.HandleFunc("/stats", serverStatsHandler)
    http.HandleFunc("/cpu", cpuHandler)
    http.ListenAndServe(":8080", nil)
}