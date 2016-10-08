package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
)

var (
	domain   string
	client   *dns.Client
	memCache *cache.Cache
	verbose  bool
)

// Question type
type Question struct {
	Name   string `json:"name"`
	Qtype  string `json:"type"`
	Qclass string `json:"class"`
}

// Hash a Question
func (q *Question) Hash() string {
	h := md5.New()
	qStr := q.Name + " " + q.Qclass + " " + q.Qtype
	h.Write([]byte(qStr))
	x := h.Sum(nil)
	key := fmt.Sprintf("%x", x)
	return key
}

func handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	q := r.Question[0]
	m := new(dns.Msg)
	question := Question{Name: strings.ToLower(q.Name), Qtype: dns.TypeToString[q.Qtype], Qclass: dns.ClassToString[q.Qclass]}

	res, ok := memCache.Get(question.Hash())
	if !ok {
		if verbose {
			log.Printf("Cache miss: %v", question)
		}
		if strings.HasSuffix(q.Name, domain) && q.Qclass == dns.ClassINET && q.Qtype == dns.TypeA {
			// Special case; Give fake response
			exp := fmt.Sprintf(`(?P<ip>(^25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)((\.|\-)(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}).%s$`, domain)
			re := regexp.MustCompile(exp)
			match := re.FindStringSubmatch(q.Name)
			if len(match) > 1 {
				match[1] = strings.Replace(match[1], "-", ".", -1)
				ip := net.ParseIP(match[1])
				if ip != nil {
					m.Question = r.Question
					m.Id = r.Id
					m.Answer = []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 3600}, A: ip.To4()}}
				}
			}
		} else {
			res, _, err := client.Exchange(r, "8.8.8.8:53")
			if err != nil {
				log.Printf("Error resolving %s", q.Name)
			} else {
				m = res
			}
		}
		memCache.Set(question.Hash(), m, cache.DefaultExpiration)
	} else {
		if verbose {
			log.Printf("Cache hit: %v", question)
		}
		m = res.(*dns.Msg)
	}

	m.Authoritative = true
	m.RecursionAvailable = true
	m.Compress = true
	m.SetReply(r)
	w.WriteMsg(m)
}

func main() {
	port := flag.Int("port", 8053, "port to run on")
	flag.BoolVar(&verbose, "verbose", false, "run in verbose mode")
	flag.StringVar(&domain, "domain", "int.example.com", "Domain to support wildcard DNS")
	flag.Parse()

	domain = domain + "."
	client = &dns.Client{
		Net:          "udp",
		ReadTimeout:  time.Duration(5) * time.Second,
		WriteTimeout: time.Duration(5) * time.Second,
	}
	memCache = cache.New(5*time.Minute, 30*time.Second)

	go func() {
		srv := &dns.Server{Addr: ":" + strconv.Itoa(*port), Net: "udp"}
		dns.HandleFunc(".", handleRequest)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set udp listener %s\n", err.Error())
		}
	}()

	go func() {
		srv := &dns.Server{Addr: ":" + strconv.Itoa(*port), Net: "tcp"}
		dns.HandleFunc(".", handleRequest)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set tcp listener %s\n", err.Error())
		}
	}()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Fatalf("Signal (%v) received, stopping\n", s)
}
