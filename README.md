# Go Concurrent Web Scraper Exercise

This project is a coding exercise demonstrating the implementation of a concurrent web scraper in Go. It's designed to showcase a Go concurrency pattern for a recursive-like function and is not intended for actual use on real APIs.

## Learning Objectives

This exercise demonstrates:

- Use of goroutines for concurrent operations instead of a traditional recursive call
- Channel usage for communication between goroutines
- Implementation of a token bucket algorithm for rate limiting
- Error handling and retry logic in concurrent scenarios
- Efficient data collection and processing in Go

## Key Components

- `initTokenBucket`: Demonstrates a rate-limiting mechanism
- `request`: Shows how to make HTTP requests and handle responses
- `recursiveReq`: Illustrates a recursive approach to breaking down work
- `worker`: Exemplifies a concurrent worker pattern

## Note

This is purely an educational exercise. The API URL used is fictional, and the code is not designed for production use. It's meant to serve as a learning tool for Go concurrency patterns.
