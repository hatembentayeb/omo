package dnscheck

import (
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"
)

func checkHTTP(domain string) *httpReport {
	rep := &httpReport{QueriedAt: time.Now()}
	target := "https://" + domain + "/"

	var hops []httpHop
	client := &http.Client{
		Timeout: httpTimeout,
		Transport: &http.Transport{
			TLSHandshakeTimeout:   dialTimeout,
			ResponseHeaderTimeout: httpTimeout,
			TLSClientConfig:       &tls.Config{ServerName: domain},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 0 {
				prev := via[len(via)-1]
				hops = append(hops, httpHop{URL: prev.URL.String(), Status: 0, Via: "redirect"})
			}
			if len(via) >= 8 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		rep.Error = err.Error()
		return tryHTTPFallback(domain, rep)
	}
	req.Header.Set("User-Agent", "omo-dnscheck/1.0")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		rep.Error = err.Error()
		return tryHTTPFallback(domain, rep)
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, 4096)

	fillHTTP(rep, resp, hops)
	return rep
}

func tryHTTPFallback(domain string, rep *httpReport) *httpReport {
	client := &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 8 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	resp, err := client.Get("http://" + domain + "/")
	if err != nil {
		if rep.Error == "" {
			rep.Error = err.Error()
		}
		return rep
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, 4096)
	fillHTTP(rep, resp, nil)
	if rep.Error != "" {
		rep.Error = "https failed; http: " + rep.Error
	}
	return rep
}

func fillHTTP(rep *httpReport, resp *http.Response, hops []httpHop) {
	rep.OK = true
	rep.Status = resp.StatusCode
	rep.FinalURL = resp.Request.URL.String()
	rep.HSTS = resp.Header.Get("Strict-Transport-Security")
	rep.Server = resp.Header.Get("Server")
	rep.CSP = resp.Header.Get("Content-Security-Policy")
	rep.XFO = firstNonEmpty(resp.Header.Get("X-Frame-Options"), resp.Header.Get("Frame-Options"))
	rep.XCTO = resp.Header.Get("X-Content-Type-Options")
	rep.Referrer = resp.Header.Get("Referrer-Policy")
	rep.Location = resp.Header.Get("Location")
	rep.ContentType = resp.Header.Get("Content-Type")

	// Reconstruct hop statuses from the redirect chain on Request.
	if len(hops) == 0 && resp.Request != nil {
		hops = append(hops, httpHop{URL: resp.Request.URL.String(), Status: resp.StatusCode, Via: "final"})
	} else {
		hops = append(hops, httpHop{URL: resp.Request.URL.String(), Status: resp.StatusCode, Via: "final"})
	}
	rep.Hops = hops

	keys := []string{
		"Strict-Transport-Security", "Server", "Content-Type", "Content-Security-Policy",
		"X-Frame-Options", "X-Content-Type-Options", "Referrer-Policy", "Permissions-Policy",
		"Access-Control-Allow-Origin", "Location", "Cache-Control", "WWW-Authenticate",
		"Expect-CT", "Report-To", "NEL", "Alt-Svc",
	}
	seen := map[string]bool{}
	for _, k := range keys {
		v := strings.Join(resp.Header.Values(k), ", ")
		if v == "" {
			continue
		}
		seen[k] = true
		rep.HeaderLines = append(rep.HeaderLines, [2]string{k, v})
	}
	for k, vals := range resp.Header {
		if seen[k] {
			continue
		}
		rep.HeaderLines = append(rep.HeaderLines, [2]string{k, strings.Join(vals, ", ")})
	}
}
