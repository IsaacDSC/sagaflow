package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {

	go server01()
	server02()

}

var counter int

func server01() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("[*] Request received: %s %s, counter: %d\n", r.Method, r.URL.Path, counter)
		if counter < 10 {
			counter++
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("Too many requests"))
			fmt.Printf("[*] Request rejected: %s %s\n", r.Method, r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello, World!"))
		fmt.Printf("[*] Request accepted: %s %s\n", r.Method, r.URL.Path)
		counter++
	})

	log.Println("server01 is running on port 3002")
	err := http.ListenAndServe(":3002", mux)
	if err != nil {
		log.Fatal(err)
	}
}

func server02() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("[*] Request received: %s %s, counter: %d\n", r.Method, r.URL.Path, counter)
		if counter < 10 {
			counter++
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("Too many requests"))
			fmt.Printf("[*] Request rejected: %s %s\n", r.Method, r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello, World!"))
		fmt.Printf("[*] Request accepted: %s %s\n", r.Method, r.URL.Path)
		counter++
	})

	log.Println("server02 is running on port 3003")
	err := http.ListenAndServe(":3003", mux)
	if err != nil {
		log.Fatal(err)
	}
}
