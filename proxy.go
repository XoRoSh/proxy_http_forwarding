package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"

	_ "github.com/mattn/go-sqlite3"
)

var customTransport = http.DefaultTransport

func cacheURL(url string) {
	db, err := sql.Open("sqlite3", "./db.db")
	if err != nil {
		fmt.Println("Error opening database")
		return
	}

	defer db.Close()

	stmt, err := db.Prepare("INSERT INTO urls (url) VALUES (?)")
	if err != nil {
		fmt.Println("Error preparing statement")
		return
	}

	_, err = stmt.Exec(url)
	if err != nil {
		fmt.Println("Error executing statement")
		return
	}

}

func isInCache(url string) bool {

	return false

}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		fmt.Println("Handling SSL request")
		handleRequestSSL(w, r)
		return
	}

	targetURL := r.URL
	fmt.Println("Handling HTTP request for URL: ", reflect.TypeOf(targetURL))

	cacheURL(r.URL.String())

	// if !isInCache(targetURL.String()) {
	// }

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

	// Copy the headers from the proxy response to the original response
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set the status code of the original response to the status code of the proxy response
	w.WriteHeader(resp.StatusCode)

	// Copy the body of the proxy response to the original response
	io.Copy(w, resp.Body)
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

func initDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./db.db")
	if err != nil {
		fmt.Println("Error opening database")
		return nil, err
	}

	schema, err := readSchema("./db.schema")

	_, err = db.Exec(schema)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return db, nil
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
