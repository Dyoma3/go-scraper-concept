package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// ############# TYPES #############

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float32 `json:"price"`
}

type ProductList struct {
	products []Product
	mu       sync.Mutex
}

type ErrorList struct {
	intervals []Interval
	mu        sync.Mutex
}

type Response struct {
	Total    int       `json:"total"`
	Count    int       `json:"count"`
	Products []Product `json:"products"`
}
type Interval [2]float32

type IntervalInfo struct {
	interval Interval
	nRetry   int
}

// ############# CONSTANTS #############

const apiURL string = "https://api.ecommerce.com/products"
const apiLimit int = 1000
const maxPrice float32 = 100000
const workerNum int = 10
const tokenBucketSize int = 10
const refreshRate time.Duration = time.Millisecond * 100

// ############# FUNCTIONS #############

func initTokenBucket(done <-chan struct{}) chan struct{} {
	tb := make(chan struct{}, tokenBucketSize)
	ticker := time.NewTicker(refreshRate)

	go func() {
		for {
			select {
			case <-ticker.C:
				select {
				case <-tb:
				case <-ticker.C:
				case <-done:
					return
				}
			case <-done:
				return
			}
		}
	}()

	return tb
}

func request(interval Interval, tokenBucket chan<- struct{}) (*Response, error) {
	params := url.Values{}
	strconv.FormatFloat(float64(interval[0]), 'f', -1, 32)
	params.Add("minPrice", strconv.FormatFloat(float64(interval[0]), 'f', -1, 32))
	params.Add("maxPrice", strconv.FormatFloat(float64(interval[1]), 'f', -1, 32))

	fullURL := apiURL + "?" + params.Encode()

	tokenBucket <- struct{}{}
	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("Error al decodificar JSON:", err)
		return nil, err
	}

	return &response, nil
}

func initialReq(tb chan struct{}) (*Response, error) {
	interval := Interval{0, maxPrice}
	res, err := request(interval, tb)
	nRetry := 0
	for err != nil && nRetry < 3 {
		res, err = request(interval, tb)
		if err == nil {
			break
		}
	}

	return res, nil
}

func recursiveReq(
	intervalInfo IntervalInfo,
	pChan chan<- Product,
	eChan chan<- Interval,
	iChan chan<- IntervalInfo,
	wg *sync.WaitGroup,
	tokenBucket chan struct{},
) {
	defer wg.Done()

	interval := intervalInfo.interval
	nRetry := intervalInfo.nRetry

	res, err := request(interval, tokenBucket)
	if err != nil {
		if nRetry == 3 {
			eChan <- interval
			return
		}
		iChan <- IntervalInfo{interval: interval, nRetry: nRetry + 1}
	}

	if res.Count < apiLimit {
		go func() {
			for _, p := range res.Products {
				pChan <- p
			}
		}()
		return
	}

	wg.Add(2)
	dif := (interval[1] - interval[0]) / 2
	iChan <- IntervalInfo{interval: Interval{interval[0], interval[0] + dif}, nRetry: 0}
	iChan <- IntervalInfo{interval: Interval{interval[0] + dif, interval[1]}, nRetry: 0}
}

func worker(
	iChan chan IntervalInfo,
	pChan chan<- Product,
	eChan chan<- Interval,
	wg *sync.WaitGroup,
	tokenBucket chan struct{},
) {
	for intInfo := range iChan {
		recursiveReq(intInfo, pChan, eChan, iChan, wg, tokenBucket)
	}
}

func getProductsList(c <-chan Product, done chan<- struct{}) *ProductList {
	pl := ProductList{products: []Product{}, mu: sync.Mutex{}}

	go func() {
		for p := range c {
			pl.mu.Lock()
			pl.products = append(pl.products, p)
			pl.mu.Unlock()
		}

		done <- struct{}{}
	}()

	return &pl
}

// Intervals that couldn't be requested
func getErrorsList(c <-chan Interval, done chan struct{}) *ErrorList {
	eList := ErrorList{intervals: []Interval{}, mu: sync.Mutex{}}

	go func() {
		for i := range c {
			eList.mu.Lock()
			eList.intervals = append(eList.intervals, i)
			eList.mu.Unlock()
		}

		done <- struct{}{}
	}()

	return &eList
}

func main() {
	pChan := make(chan Product, 1000)
	eChan := make(chan Interval, 100)
	iChan := make(chan IntervalInfo, 100)
	done := make(chan struct{})

	tb := initTokenBucket(done)
	wg := sync.WaitGroup{}

	// Initial request to make estimation of intervals
	res, err := initialReq(tb)
	if err != nil {
		log.Fatal(err)
	}

	nIntervals := res.Total / apiLimit
	intLen := maxPrice / float32(nIntervals)
	interval := Interval{0, intLen}

	wg.Add(nIntervals)
	for i := 0; i < nIntervals; i++ {
		iChan <- IntervalInfo{interval: interval, nRetry: 0}
		interval[0], interval[1] = interval[1], interval[1]+intLen
	}

	for i := 0; i < workerNum; i++ {
		go worker(iChan, pChan, eChan, &wg, tb)
	}

	listsDone := make(chan struct{}, 2)
	pl := getProductsList(pChan, listsDone)
	el := getErrorsList(eChan, listsDone)

	wg.Wait()
	done <- struct{}{}
	close(done)
	close(iChan)
	close(pChan)
	close(eChan)
	<-listsDone
	<-listsDone
	close(listsDone)

	for _, p := range pl.products {
		fmt.Println(p)
	}
	for _, i := range el.intervals {
		fmt.Println(i)
	}
}
