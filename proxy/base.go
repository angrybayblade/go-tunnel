package proxy

import (
	"strconv"
)

const MaxConnectionPoolSize int = 5
const HttpRequestPipeChunkSize int = 64

type Addr struct {
	Host string
	Port int
}

func (a *Addr) ToString() string {
	return a.Host + ":" + strconv.Itoa(a.Port)
}
