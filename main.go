package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
)

var (
	socketPath = flag.String("socket", "/var/run/tailscale/tailscaled.sock", "path to Tailscale UNIX socket")
	skipOrigin = flag.String("skip-origin", "127.0.0.1/32", "CIDR range to skip auth, i.e. always allowed")
	listen     = flag.String("listen", "127.0.0.1:9466", "Bind address to listen for requests")
)

type TailscaleUserProfile struct {
	ID            int64
	LoginName     string
	DisplayName   string
	ProfilePicURL string
}

func main() {
	flag.Parse()

	cidrOrigin, err := netip.ParsePrefix(*skipOrigin)
	if err != nil {
		panic(err)
	}

	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", *socketPath)
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		log.Printf("[%s->%s] %s", r.RemoteAddr, r.Host, ip)
		reqOrigin, err := netip.ParseAddr(ip)
		if err != nil {
			w.WriteHeader(500)
			log.Printf("error: %v", err)
			return
		}
		if cidrOrigin.Contains(reqOrigin) {
			w.WriteHeader(204)
			w.Header().Set("Tailscale-User-Login", r.RemoteAddr)
			w.Header().Set("Tailscale-User-Name", "anonymous")
			log.Printf("match skip origin: %s", *skipOrigin)
			return
		}

		// Port seems like magic number that just needs to be there?
		// Hostname is hardcoded:
		// https://github.com/tailscale/tailscale/blob/99b9d7a621c8f094f83bf56b716e6d29dbebbc01/ipn/localapi/localapi.go#L187-L209
		url := fmt.Sprintf("http://local-tailscaled.sock/localapi/v0/whois?addr=%s:12345", ip)

		response, err := httpc.Get(url)
		if err != nil {
			w.WriteHeader(500)
			log.Printf("error: %v", err)
			return
		}

		if response.StatusCode != 200 {
			w.WriteHeader(403)
			var buf bytes.Buffer
			io.Copy(&buf, response.Body)
			log.Printf("unsuccessful auth: %+v", buf.String())
			return
		}

		var body map[string]TailscaleUserProfile
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			w.WriteHeader(500)
			log.Printf("error: %v", err)
			return
		}

		user := body["UserProfile"]

		log.Printf("IP: %v, ID: %v, Name: %v", ip, user.ID, user.DisplayName)

		w.Header().Set("Tailscale-User-Login", user.LoginName)
		w.Header().Set("Tailscale-User-Name", user.DisplayName)

		w.WriteHeader(204)
	})

	log.Printf("listening: %v", *listen)
	log.Fatal(http.ListenAndServe(*listen, mux))
}
