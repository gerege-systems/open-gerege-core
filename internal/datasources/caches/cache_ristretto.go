// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package caches

import (
	"time"

	ristr "github.com/dgraph-io/ristretto"
)

// defaultRistrettoTTL нь кэшэлсэн бичлэгүүдийн аюулгүйн сүлжээ болсон
// хүчингүй болох хугацаа юм. Тодорхой хүчингүй болголт (жишээ нь
// Activate / UpdatePassword дээр) нь мэдэгдэж буй мутацийн замуудыг
// хамардаг боловч TTL нь бидний холбож амжаагүй ирээдүйн ямар нэг
// мутацаас хамгаална — хуучирсан өгөгдөл мөнхөд оршихын оронд
// хугацаа дуусч устдаг.
const defaultRistrettoTTL = 5 * time.Minute

type RistrettoCache interface {
	// Set нь value-г key дор cost 1 болон package-ийн өгөгдмөл TTL-тэйгээр
	// хадгална. Бичилт нь async — утга нь дараагийн Get-д шууд
	// харагдахгүй байж магадгүй.
	Set(key string, value interface{})
	// Get нь кэшэлсэн утгыг буцаана, эсвэл олдоогүй / төрөл таарахгүй үед
	// nil-г буцаана. Дуудагчид type-assert хийх ёстой.
	Get(key string) interface{}
	// Del нь нэг буюу олон key-г устгана. Байхгүй key нь алдаа биш.
	Del(key ...string)
}

type ristrettoCache struct {
	cache *ristr.Cache
}

func NewRistrettoCache() (RistrettoCache, error) {
	cache, err := ristr.NewCache(&ristr.Config{
		BufferItems: 64,
		MaxCost:     1 << 30,
		NumCounters: 1e7,
	})
	if err != nil {
		return nil, err
	}

	return &ristrettoCache{cache: cache}, nil
}

func (cache *ristrettoCache) Set(key string, value interface{}) {
	cache.cache.SetWithTTL(key, value, 1, defaultRistrettoTTL)
}

func (cache *ristrettoCache) Get(key string) interface{} {
	val, _ := cache.cache.Get(key)

	return val
}

func (cache *ristrettoCache) Del(key ...string) {
	for _, i := range key {
		cache.cache.Del(i)
	}
}
