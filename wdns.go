package main

import (
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
)

var (
	domain string
	client *dns.Client
)

func handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	q := r.Question[0]
	m := new(dns.Msg)

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

	m.Authoritative = true
	m.RecursionAvailable = true
	m.Compress = true
	m.SetReply(r)
	w.WriteMsg(m)
}

func main() {
	port := flag.Int("port", 8053, "port to run on")
	flag.StringVar(&domain, "domain", "int.example.com", "Domain to support wildcard DNS")
	flag.Parse()

	domain = domain + "."
	client = &dns.Client{
		Net:          "udp",
		ReadTimeout:  time.Duration(5) * time.Second,
		WriteTimeout: time.Duration(5) * time.Second,
	}

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
