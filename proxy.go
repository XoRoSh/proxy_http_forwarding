package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var customTransport = http.DefaultTransport
var db *sql.DB

func cacheResponse(url string, method string, requestHeaders, responseHeaders, responseBody string, statusCode int) {

	headers := http.Header{}
	for _, header := range strings.Split(responseHeaders, "\n") {
		if header == "" {
			continue
		}
		parts := strings.SplitN(header, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		headers.Add(parts[0], parts[1])
	}

	headersJSON, err := json.Marshal(headers)
	if err != nil {
		log.Println("Error encoding headers to JSON:", err)
		return
	}

	stmt, err := db.Prepare("INSERT INTO cache (url, method, request_headers, response_headers, response_body, status_code) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Println("Error preparing statement:", err)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(url, method, requestHeaders, string(headersJSON), responseBody, statusCode)
	if err != nil {
		log.Println("Error executing statement:", err)
		return
	}
	log.Println("Response cached for:", url)
}

func isInCache(url string) bool {

	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM cache WHERE url = ?)", url).Scan(&exists)
	if err != nil {
		log.Println("Error checking if is in cache database:", err)
	}
	fmt.Println("Cached response found for URL:", url)
	return exists
}

func cachedResponseIfIsInCache(url string, w http.ResponseWriter) {

	var responseHeaders, responseBody string
	var statusCode int
	err := db.QueryRow("SELECT response_headers, response_body, status_code FROM cache WHERE url = ?", url).Scan(&responseHeaders, &responseBody, &statusCode)
	if err != nil {
		log.Println("Error querying cached response:", err)
		return
	}

	// Write the cached response headers to the original response
	headers := http.Header{}
	for _, header := range strings.Split(responseHeaders, "\n") {
		if header == "" {
			continue
		}
		parts := strings.SplitN(header, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		headers.Add(parts[0], parts[1])
	}

	for name, values := range headers {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Write the cached response body to the original response
	w.WriteHeader(statusCode)
	w.Write([]byte(responseBody))
}

func isBlacklisted(url string) bool {

	var blacklisted bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM cache WHERE url = ? AND blacklist = 1)", url).Scan(&blacklisted)
	if err != nil {
		log.Println("Error checking if URL is blacklisted:", err)
		return false
	}
	return blacklisted
}
func handleRequest(w http.ResponseWriter, r *http.Request) {

	if isBlacklisted(r.URL.String()) {
		http.Error(w, "URL is blacklisted", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodConnect {
		fmt.Println("Handling SSL request")
		handleRequestSSL(w, r)
		return
	}

	targetURL := r.URL

	if isInCache(targetURL.String()) {
		cachedResponseIfIsInCache(targetURL.String(), w)
		return
	}

	print("Request URL: ", targetURL.String())
	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	fmt.Print("Request URL: ", targetURL.String())
	if err != nil {
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		return
	}

	// Copy the headers from the original request to the proxy request
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Send the proxy request using the custom transport
	resp, err := customTransport.RoundTrip(proxyReq)
	if err != nil {
		http.Error(w, "Error sending proxy request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading response body", http.StatusInternalServerError)
		return
	}

	resp.Body = io.NopCloser(strings.NewReader(string(responseBody)))

	// Cache the response
	cacheResponse(targetURL.String(), r.Method, headersToJSON(r.Header), headersToJSON(resp.Header), string(responseBody), resp.StatusCode)

	// Copy the headers from the proxy response to the original response
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set the status code of the original response to the status code of the proxy response
	w.WriteHeader(resp.StatusCode)
	w.Write(responseBody)

	// Copy the body of the proxy response to the original response
	// io.Copy(w, resp.Body)
}

func headersToJSON(headers http.Header) string {
	jsonData, err := json.Marshal(headers)
	if err != nil {
		return "{}" // Return empty JSON object on error
	}
	return string(jsonData)
}

func handleRequestSSL(w http.ResponseWriter, r *http.Request) {
	// Extract the host and port from the request URL
	host := r.URL.Host
	fmt.Print("Host: ", host)
	if host == "" {
		http.Error(w, "Invalid host", http.StatusBadRequest)
		return
	}

	// check that URL's in the blacklist

	// Establish a TCP connection to the host
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "Error hijacking connection", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Send a 200 OK response to the client to establish the connection
	clientConn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

	// Establish a TCP connection to the host
	serverConn, err := net.Dial("tcp", host)
	if err != nil {
		http.Error(w, "Error establishing server connection", http.StatusInternalServerError)
		return
	}
	defer serverConn.Close()

	// Copy data between the client and server
	go io.Copy(serverConn, clientConn)
	io.Copy(clientConn, serverConn)
}

func readSchema(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./db.db")
	if err != nil {
		log.Fatal("Error opening database")
	}

	_, err = db.Exec("DROP TABLE IF EXISTS cache")
	if err != nil {
		log.Fatal("Error reading schema:", err)
	}

	schema, err := readSchema("./db.schema")
	if err != nil {
		log.Fatal("Error reading schema:", err)
	}

	_, err = db.Exec(schema)
	if err != nil {
		log.Fatal("Error applying schema:", err)
	}
}

func main() {
	// Create a new HTTP server with the handleRequest function as the handler
	initDB()
	server := http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(handleRequest),
	}

	// Start the server and log any errors
	log.Println("Starting proxy server on :8080")
	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("Error starting proxy server: ", err)
	}
}
