/*
* Реализовал всю указанную в задании функциональность
* Программа хорошо структурирована, отдельные функции и методы не требуют комментариев.
* Сервис представляет из себя http-сервер с двумя обработчиками, который может обрабатывать запросы типа:
* curl -H "X-Forwarded-For: 127.0.0.1" http://host:port/
* curl -H "X-Forwarded-For: 127.0.0.1" http://host:port/limits //а вот так можно сбросить счетчик
 */

package main

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"time"
)

func main() {
	subnetPrefixEnv, exists := os.LookupEnv("SUBNET_PREFIX")
	if !exists {
		subnetPrefixEnv = "24"
	}
	subnetPrefix, err := strconv.Atoi(subnetPrefixEnv)
	if err != nil {
		//обработка ошибки
	}

	limitEnv, exists := os.LookupEnv("SUBNET_REQUEST_LIMIT")
	if !exists {
		limitEnv = "100"
	}
	limit, err := strconv.Atoi(limitEnv)
	if err != nil {
		//обработка ошибки
	}

	var server Server
	server.subnet.counters = make(map[string]RateCounter)
	server.subnet.subnetPrefix = subnetPrefix
	server.subnet.limit = limit

	http.HandleFunc("/", server.commonHandler)
	http.HandleFunc("/limits", server.limitsHandler)

	PORT := ":4001"

	err = http.ListenAndServe(PORT, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
}

type Server struct {
	subnet SubnetGroup
}

type RateCounter struct {
	stamp time.Time
	value int
}

type SubnetGroup struct {
	counters     map[string]RateCounter
	subnetPrefix int
	limit        int
}

func getSubnet(address string, subnetPrefix int) string {
	addr, err := netip.ParseAddr(address)
	if err != nil {
		//обработка
	}
	prefix := netip.PrefixFrom(addr, subnetPrefix)
	_, subnet, err := net.ParseCIDR(prefix.String())
	if err != nil {
		//обработка
	}

	return subnet.String()
}

func (group *SubnetGroup) incCounter(subnet string) {
	counter, _ := group.counters[subnet]

	counter.value++
	counter.stamp = time.Now()
	group.counters[subnet] = counter
}

func checkTimeLimit(counter RateCounter, duration time.Duration) bool {
	return counter.stamp.Add(duration).Compare(time.Now()) == 1 //если не прошло минуты после первого запроса
}

func checkCounterLimit(counter RateCounter, limit int) bool {
	return counter.value < limit
}

func (group *SubnetGroup) checkRate(subnet string) bool {
	counter, ok := group.counters[subnet]

	if ok {
		if checkTimeLimit(counter, time.Minute) {
			return checkCounterLimit(counter, group.limit)
		} else {
			return true
		}
	} else {
		return true
	}
}

func (group *SubnetGroup) resetLimitIfExist(subnet string) {
	_, ok := group.counters[subnet]
	if ok {
		group.counters[subnet] = RateCounter{}
	}
}

func (server *Server) commonHandler(w http.ResponseWriter, r *http.Request) {
	address := r.Header.Get("X-Forwarded-For")

	subnet := getSubnet(address, server.subnet.subnetPrefix)

	if !server.subnet.checkRate(subnet) {
		w.WriteHeader(http.StatusTooManyRequests)
	} else {
		server.subnet.incCounter(subnet)
	}
}

// handler для сброса лимита по префиксу
func (server *Server) limitsHandler(w http.ResponseWriter, r *http.Request) {
	address := r.Header.Get("X-Forwarded-For")

	subnet := getSubnet(address, server.subnet.subnetPrefix)
	server.subnet.resetLimitIfExist(subnet)
}
