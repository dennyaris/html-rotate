package util

import (
	"fmt"
	"log"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

var mc *memcache.Client

func InitMemcached(addr string) {
	mc = memcache.New(addr)
}

func SetMemcachedValue(key string, value []byte, expiration int) error {
	// Create a time.Duration for expiration time in seconds
	expirationTime := time.Duration(expiration) * time.Second
	// Get the total seconds as int64
	expirationInSeconds := int32(expirationTime.Seconds())
	log.Printf("set expired time: %d seconds", expirationInSeconds)

	// Set the value in the cache with the specified key, value, and expiration time
	return mc.Set(&memcache.Item{Key: key, Value: value, Expiration: expirationInSeconds})
}

func GetMemcachedValue(key string) ([]byte, error) {
	item, err := mc.Get(key)
	if err != nil {
		if err == memcache.ErrCacheMiss {
			return nil, fmt.Errorf("key not found in memcached: %s", key)
		}
		return nil, fmt.Errorf("error getting value from memcached: %v", err)
	}

	return item.Value, nil
}

func Flush() {
	mc.FlushAll()
}

func UpdateValueMemcached(key string, value []byte, expiration int) (*memcache.Item, error) {
	expirationTime := time.Duration(expiration) * time.Second
	expirationInSeconds := int32(expirationTime.Seconds())

	newItem := memcache.Item{
		Key:        key,
		Value:      value,
		Expiration: expirationInSeconds,
	}

	if err := mc.Set(&newItem); err != nil {
		return nil, err
	}

	return &newItem, nil
}
