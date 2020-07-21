package main

import (
	"log"
	"math/rand"
	"net/url"
	"time"
)

// Seed
var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

// String charset to randomly pick from
const RandomStringCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Return a random string of the given length
func getRandString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = RandomStringCharset[seededRand.Intn(len(RandomStringCharset))]
	}
	return string(b)
}

// Get the elapsed milliseconds from a given starting point
func getElapsedTimeInMS(start int64) int64 {
	return (time.Now().UnixNano() - start) / int64(time.Millisecond)
}

// isValidUrl tests a string to determine if it is a well-structured url or not.
func isValidUrl(toTest string) (*url.URL, bool) {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return nil, false
	}

	u, err := url.Parse(toTest)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, false
	}

	return u, true
}

// Contains a string in a slice
func contains(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

// If a critical error pops up, fail
func checkFatalError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
