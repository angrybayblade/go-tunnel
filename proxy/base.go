package proxy

import (
	"strconv"
)

const MaxConnectionPoolSize int = 5

type Addr struct {
	Host string
	Port int
}

func (a *Addr) ToString() string {
	return a.Host + ":" + strconv.Itoa(a.Port)
}
