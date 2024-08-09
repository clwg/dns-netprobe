package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/miekg/dns"
)

type DnsQuery struct {
	Timestamp time.Time `db:"timestamp"`
	Ip        string    `db:"ip"`
	Domain    string    `db:"domain"`
	Query     string    `db:"query"`
	Answer    string    `db:"answer"`
}

const schema = `
CREATE TABLE IF NOT EXISTS dns_queries (
    timestamp TIMESTAMP,
    ip TEXT,
    domain TEXT,
    query TEXT,
    answer TEXT
);
`

func main() {
	domain := flag.String("domain", "", "Domain to query")
	network := flag.String("network", "", "Network range to query")
	timeout := flag.Int("timeout", 5, "Timeout for DNS queries in seconds")
	domains := flag.String("domains", "", "Comma-separated list of additional domains to query")
	dbfile := flag.String("db", "dns.db", "SQLite database file")
	goroutineLimit := flag.Int("concurrent", 256, "Limit for concurrent goroutines")
	flag.Parse()

	db, err := sqlx.Open("sqlite3", *dbfile)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(*dbfile); os.IsNotExist(err) {
		_, err = db.Exec(schema)
		if err != nil {
			log.Fatalf("Failed to create table: %v", err)
		}
	}

	client := dns.Client{Timeout: time.Duration(*timeout) * time.Second}

	ip, ipnet, err := net.ParseCIDR(*network)
	if err != nil {
		log.Fatalf("Failed to parse network range: %v", err)
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, *goroutineLimit) // Limit concurrent goroutines

	stmt, err := db.Preparex("INSERT INTO dns_queries (timestamp, ip, domain, query, answer) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatalf("Failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incrementIP(ip) {
		wg.Add(1)
		semaphore <- struct{}{} // acquire a slot in the semaphore

		go func(ip net.IP) {
			defer wg.Done()
			defer func() { <-semaphore }() // release slot in the semaphore when goroutine is done

			performDnsQuery(client, stmt, ip, *domain)

			if *domains != "" {
				for _, additionalDomain := range strings.Split(*domains, ",") {
					performDnsQuery(client, stmt, ip, additionalDomain)
				}
			}
		}(append(ip[:0:0], ip...))
	}

	wg.Wait()
}

func performDnsQuery(client dns.Client, stmt *sqlx.Stmt, ip net.IP, domain string) {
	msg := dns.Msg{}
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	resp, _, err := client.Exchange(&msg, net.JoinHostPort(ip.String(), "53"))
	if err != nil {
		//log.Printf("Query request timeout for IP %s: %s\n", ip.String(), err)
		return
	}

	query := dnsQuestionToString(msg.Question[0])
	answer := dnsRRToString(resp.Answer)

	_, err = stmt.Exec(time.Now(), ip.String(), domain, query, answer)
	if err != nil {
		log.Printf("Failed to execute statement for IP %s: %v", ip.String(), err)
	}
}

func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

func dnsQuestionToString(q dns.Question) string {
	return fmt.Sprintf("%s %s", q.Name, dns.TypeToString[q.Qtype])
}

func dnsRRToString(rr []dns.RR) string {
	var sb strings.Builder
	for _, r := range rr {
		sb.WriteString(r.String() + "\n")
	}
	return sb.String()
}
