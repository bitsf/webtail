package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
)

var (
	flagT = flag.Int64("t", 5, "refresh time")
	flagS = flag.Int64("s", 0, "skip body byte")
	flagC = flag.Int64("c", 600, "max no data wait seconds")
	flagM = flag.Int64("m", 1000000, "max body size")
	flagF = flag.Bool("f", false, "follow log")
	flagL = flag.Int64("l", -1, "last body byte, override -s")
	flagA = flag.Bool("a", false, "show animation icon")
)

func init() {
	flag.Usage = func() {
		os.Stderr.WriteString(`tail the log from web using http Range
Usage:
	$ webtail <url>
`)
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	u := flag.Args()[0]
	c := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 5 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,

			ExpectContinueTimeout: 4 * time.Second,
			ResponseHeaderTimeout: 3 * time.Second,
		},
		// Prevent endless redirects
		Timeout: 1 * time.Minute,
	}
	var sp *spinner.Spinner
	if *flagA{
		sp = spinner.New(spinner.CharSets[9], 500*time.Millisecond)
	}
	fmt.Printf("query: %s\n", u)
	var cur = *flagS
	if *flagL >= 0 {
		if *flagL == 0 {
			*flagL = 1
		}
		if r, e := c.Head(u); e != nil {
			panic(e)
		} else {
			cur = r.ContentLength - *flagL
		}
		if cur < 0 {
			cur = 0
		}
	}

	var nodatatime int64
	for {
		req, e := http.NewRequest(http.MethodGet, u, nil)
		if e != nil {
			panic(e)
		}
		if *flagM <= 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", cur))
		}else {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", cur, cur+*flagM-1))
		}
		if *flagA {
			sp.UpdateCharSet(spinner.CharSets[39])
			sp.Start()
		}
		if r, e := c.Do(req); e != nil {
			panic(e)
		} else {
			if *flagA {
				sp.Stop()
			}
			if r.StatusCode != http.StatusRequestedRangeNotSatisfiable {
				if n, e := io.Copy(os.Stdout, r.Body); e != nil {
					//fmt.Println(e, n)
					if strings.Contains(e.Error(), "Client.Timeout exceeded") {
						if r.Header.Get("Content-Range") != "" {
							cur += n
							if n == 0 {
								if nodatatime > *flagC {
									fmt.Println("no data after ", nodatatime)
									break
								}
								nodatatime += *flagT
							} else {
								nodatatime = 0
							}
						} else {
							break
						}
					} else {
						fmt.Println(e, n)
						break
					}
				} else {
					if r.Header.Get("Content-Range") != "" {
						if !*flagF {
							break
						}
						cur += n
						if n == 0 {
							if nodatatime > *flagC {
								fmt.Println("no data after ", nodatatime)
								break
							}
							nodatatime += *flagT
						} else {
							nodatatime = 0
						}
					} else {
						break
					}
				}
			}
		}
		if *flagA {
			sp.UpdateCharSet(spinner.CharSets[9])
			sp.Start()
		}
		time.Sleep(time.Second * time.Duration(*flagT))
		if *flagA {
			sp.Stop()
		}
	}
}
