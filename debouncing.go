package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Status string

const (
	PRESENT Status = "present"
	FETCHING Status = "fetching"
	ABSENT Status = "absent"
)

type KeyMeta struct {
	lock 	 sync.Mutex
	waitCond *sync.Cond
	status Status
}

type CacheLock struct {
	lock, rwlock, sfLock sync.Mutex
	rc int
}

var cacheLock = &CacheLock{
	lock: sync.Mutex{},
	rwlock: sync.Mutex{},
	// lock to prevent starvation of writers i.e. if a writer is waiting, no new readers will be allowed before it writes
	sfLock: sync.Mutex{},
	rc: 0,
}

var cache = make(map[int]string)

var meta = make(map[int]*KeyMeta) 

var keyMetaMutex = sync.Mutex{}

func getOrCreateKeyMeta(id int) *KeyMeta {
	keyMetaMutex.Lock()
	defer keyMetaMutex.Unlock()
	if keyMetaPointer, exists := meta[id]; !exists {
		keyMetaPointer = &KeyMeta{lock: sync.Mutex{}}
		keyMetaPointer.waitCond = sync.NewCond(&keyMetaPointer.lock)
		keyMetaPointer.status = ABSENT
		meta[id] = keyMetaPointer
		return keyMetaPointer
	} else {
		return keyMetaPointer
	}
}

var fetchedFromDB, fetchedFromCache atomic.Int32

func fetchFromDB(key int) string {
	fetchedFromDB.Add(1)
	time.Sleep(1000*time.Millisecond) // simulate fetch from db
	return fmt.Sprintf("data for key %d", key)
}

func readFromCache(key int) (string, bool) {
	cacheLock.sfLock.Lock()
	cacheLock.lock.Lock()
	cacheLock.rc++
	if cacheLock.rc == 1 {
		cacheLock.rwlock.Lock()
	}
	cacheLock.lock.Unlock()
	cacheLock.sfLock.Unlock()
	time.Sleep(50*time.Millisecond) // simulate reading from remote cache or disk cache
	data, exists := cache[key]
	cacheLock.lock.Lock()
	cacheLock.rc--
	if cacheLock.rc == 0 {
		cacheLock.rwlock.Unlock()
	} 
	cacheLock.lock.Unlock()
	if exists {
		fetchedFromCache.Add(1)
	}
	return data, exists
}

func writeToCache(key int, value string) {
	cacheLock.sfLock.Lock()
	cacheLock.rwlock.Lock()
	time.Sleep(50*time.Millisecond) // simulate writing to remote cache or disk cache
	cache[key] = value
	cacheLock.rwlock.Unlock()
	cacheLock.sfLock.Unlock()
} 

func fetchDataDebounce(i int, key int) string {

	keyMetaPointer := getOrCreateKeyMeta(key)
	keyMetaPointer.lock.Lock()
	if keyMetaPointer.status == PRESENT {
		//fmt.Printf("request %d: data for key %d is present\n", i, key)
		defer keyMetaPointer.lock.Unlock()
		data, _ := readFromCache(key)
		return data
	} else if keyMetaPointer.status == FETCHING {
		//fmt.Printf("request %d: waiting to fetch data for key %d\n", i, key)
		keyMetaPointer.waitCond.Wait()
		defer keyMetaPointer.lock.Unlock()
		data, _ := readFromCache(key)
		return data
	} else {
		//fmt.Printf("request %d: fetching data for key %d\n", i, key)
		keyMetaPointer.status = FETCHING
		keyMetaPointer.lock.Unlock()
		data := fetchFromDB(key)
		writeToCache(key, data)
		keyMetaPointer.lock.Lock()
		keyMetaPointer.status = PRESENT
		keyMetaPointer.lock.Unlock()
		//fmt.Printf("request %d: fetched data for key %d\nGoing to wakeup other requests\n", i, key)
		keyMetaPointer.waitCond.Broadcast()
		return data
	}
}

type KeyLock struct {
	lock, rwlock, starvationFreeLock sync.Mutex
	readcount int
}
var cacheKeyLocks = make(map[int]*KeyLock)

var keyLocksMutex = sync.Mutex{}

func getOrCreateKeyLocks(key int) *KeyLock {
	keyLocksMutex.Lock()
	defer keyLocksMutex.Unlock()
	if keyLockPtr, exists := cacheKeyLocks[key]; !exists {
		keyLockPtr = &KeyLock{
			lock: sync.Mutex{},
			rwlock: sync.Mutex{},
			starvationFreeLock: sync.Mutex{},
			readcount: 0,
		}
		cacheKeyLocks[key] = keyLockPtr
		return keyLockPtr
	} else {
		return keyLockPtr
	}
}

func fetchData(id, key int) string {
	if data, exists := readFromCache(key); !exists {
		data = fetchFromDB(key)
		writeToCache(key, data)
		return data
	} else {
		// fmt.Printf("request %d: found in cache for key %d\n", id, key) 
		return data
	}
}

func main() {

	var wg sync.WaitGroup 
	numRequests := 100
	start := time.Now()
	for i :=0; i<numRequests; i++ {
		wg.Add(1)
		time.Sleep(50*time.Millisecond) // to simulate requests coming in at some intervals apart
		go func(i int) {
			fetchDataDebounce(i, 1 + i%2)
			//data := fetchDataDebounce(i, 1 + i%2)
			// data := fetchData(i, 1 + i%2)
			// fetchData(i, 1 + i%2)
			// fmt.Printf("fetched data for req %d and key %d: %v\n", i, 1 + i%2, data)
			wg.Done()
		}(i)
	}
	wg.Wait()
	fmt.Printf("Fetched items from DB = %v\nFetched items from cache = %v\n", fetchedFromDB.Load(), fetchedFromCache.Load())
	fmt.Printf("Time Taken - %v\n", time.Since(start))
}