package bunnydns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"omo/pkg/pluginapi"
)

// Client talks to api.bunny.net DNS Zone APIs.
type Client struct {
	accessKey string
	baseURL   string
	http      *http.Client
}

func NewClient(accessKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = apiBaseDefault
	}
	return &Client{
		accessKey: accessKey,
		baseURL:   strings.TrimRight(baseURL, "/"),
		http:      pluginapi.NewHTTPClient(45 * time.Second),
	}
}

func (c *Client) Connected() bool {
	return c != nil && c.accessKey != ""
}

type paginated[T any] struct {
	Items        []T  `json:"Items"`
	CurrentPage  int  `json:"CurrentPage"`
	TotalItems   int  `json:"TotalItems"`
	HasMoreItems bool `json:"HasMoreItems"`
}

type DnsZone struct {
	Id                       int64       `json:"Id"`
	Domain                   string      `json:"Domain"`
	Records                  []DnsRecord `json:"Records"`
	DateModified             string      `json:"DateModified"`
	DateCreated              string      `json:"DateCreated"`
	NameserversDetected      bool        `json:"NameserversDetected"`
	CustomNameserversEnabled bool        `json:"CustomNameserversEnabled"`
	Nameserver1              string      `json:"Nameserver1"`
	Nameserver2              string      `json:"Nameserver2"`
	SoaEmail                 string      `json:"SoaEmail"`
	LoggingEnabled           bool        `json:"LoggingEnabled"`
	DnsSecEnabled            bool        `json:"DnsSecEnabled"`
}

type DnsRecord struct {
	Id            int64  `json:"Id"`
	Type          int    `json:"Type"`
	Ttl           int    `json:"Ttl"`
	Value         string `json:"Value"`
	Name          string `json:"Name"`
	Weight        int    `json:"Weight"`
	Priority      int    `json:"Priority"`
	Port          int    `json:"Port"`
	Flags         int    `json:"Flags"`
	Tag           string `json:"Tag"`
	Disabled      bool   `json:"Disabled"`
	Comment       string `json:"Comment"`
	Accelerated   bool   `json:"Accelerated"`
	MonitorStatus int    `json:"MonitorStatus"`
}

type DnsSecInfo struct {
	Enabled      bool   `json:"Enabled"`
	DsRecord     string `json:"DsRecord"`
	Digest       string `json:"Digest"`
	DigestType   string `json:"DigestType"`
	Algorithm    int    `json:"Algorithm"`
	PublicKey    string `json:"PublicKey"`
	KeyTag       int    `json:"KeyTag"`
	Flags        int    `json:"Flags"`
	DsConfigured bool   `json:"DsConfigured"`
}

type DnsStats struct {
	TotalQueriesServed         int64              `json:"TotalQueriesServed"`
	QueriesServedChart         map[string]float64 `json:"QueriesServedChart"`
	NormalQueriesServedChart   map[string]float64 `json:"NormalQueriesServedChart"`
	SmartQueriesServedChart    map[string]float64 `json:"SmartQueriesServedChart"`
	QueriesByTypeChart         map[string]int64   `json:"QueriesByTypeChart"`
}

type CheckAvailability struct {
	Available bool   `json:"Available"`
	Message   string `json:"Message"`
}

func (c *Client) ListZones(search string) ([]DnsZone, error) {
	q := url.Values{"page": {"1"}, "perPage": {"1000"}}
	if search != "" {
		q.Set("search", search)
	}
	var out paginated[DnsZone]
	if err := c.doJSON(http.MethodGet, "/dnszone?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

func (c *Client) GetZone(id int64) (*DnsZone, error) {
	var z DnsZone
	if err := c.doJSON(http.MethodGet, fmt.Sprintf("/dnszone/%d", id), nil, &z); err != nil {
		return nil, err
	}
	return &z, nil
}

func (c *Client) AddZone(domain string) (*DnsZone, error) {
	var z DnsZone
	if err := c.doJSON(http.MethodPost, "/dnszone", map[string]any{"Domain": domain}, &z); err != nil {
		return nil, err
	}
	return &z, nil
}

func (c *Client) DeleteZone(id int64) error {
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/dnszone/%d", id), nil, nil)
}

func (c *Client) UpdateZone(id int64, patch map[string]any) error {
	return c.doJSON(http.MethodPost, fmt.Sprintf("/dnszone/%d", id), patch, nil)
}

func (c *Client) ListRecords(zoneID int64, typeFilter, search string) ([]DnsRecord, error) {
	q := url.Values{"page": {"1"}, "perPage": {"1000"}}
	if typeFilter != "" {
		if code, ok := parseRecordType(typeFilter); ok {
			q.Set("type", strconv.Itoa(code))
		}
	}
	if search != "" {
		q.Set("search", search)
	}
	var out paginated[DnsRecord]
	if err := c.doJSON(http.MethodGet, fmt.Sprintf("/dnszone/%d/records?%s", zoneID, q.Encode()), nil, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

func (c *Client) AddRecord(zoneID int64, rec map[string]any) (*DnsRecord, error) {
	var out DnsRecord
	if err := c.doJSON(http.MethodPut, fmt.Sprintf("/dnszone/%d/records", zoneID), rec, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateRecord(zoneID, recordID int64, rec map[string]any) error {
	return c.doJSON(http.MethodPost, fmt.Sprintf("/dnszone/%d/records/%d", zoneID, recordID), rec, nil)
}

func (c *Client) DeleteRecord(zoneID, recordID int64) error {
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/dnszone/%d/records/%d", zoneID, recordID), nil, nil)
}

func (c *Client) ExportZone(zoneID int64) (string, error) {
	data, err := c.doRaw(http.MethodGet, fmt.Sprintf("/dnszone/%d/export", zoneID), nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *Client) EnableDNSSEC(zoneID int64) (*DnsSecInfo, error) {
	var out DnsSecInfo
	if err := c.doJSON(http.MethodPost, fmt.Sprintf("/dnszone/%d/dnssec", zoneID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DisableDNSSEC(zoneID int64) error {
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/dnszone/%d/dnssec", zoneID), nil, nil)
}

func (c *Client) GetStatistics(zoneID int64) (*DnsStats, error) {
	var out DnsStats
	if err := c.doJSON(http.MethodGet, fmt.Sprintf("/dnszone/%d/statistics", zoneID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CheckAvailability(domain string) (*CheckAvailability, error) {
	q := url.Values{"domain": {domain}}
	var out CheckAvailability
	if err := c.doJSON(http.MethodGet, "/dnszone/check?"+q.Encode(), nil, &out); err != nil {
		if err2 := c.doJSON(http.MethodGet, "/dnszone/availability?"+q.Encode(), nil, &out); err2 != nil {
			return nil, err
		}
	}
	return &out, nil
}

func (c *Client) TriggerScan(zoneID int64, domain string) error {
	body := map[string]any{}
	if zoneID > 0 {
		body["ZoneId"] = zoneID
	}
	if domain != "" {
		body["Domain"] = domain
	}
	return c.doJSON(http.MethodPost, "/dnszone/scan", body, nil)
}

func (c *Client) GetScanResult(zoneID int64) (json.RawMessage, error) {
	data, err := c.doRaw(http.MethodGet, fmt.Sprintf("/dnszone/%d/scan", zoneID), nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (c *Client) IssueWildcardCert(zoneID int64) error {
	return c.doJSON(http.MethodPost, fmt.Sprintf("/dnszone/%d/certificate", zoneID), nil, nil)
}

func parseRecordType(s string) (int, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, true
	}
	n, ok := dnsRecordTypeCodes[s]
	return n, ok
}

func (c *Client) doJSON(method, path string, body any, out any) error {
	data, err := c.doRaw(method, path, body)
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) doRaw(method, path string, body any) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("AccessKey", c.accessKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
		var apiErr struct {
			Message  string `json:"Message"`
			ErrorKey string `json:"ErrorKey"`
		}
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Message != "" {
			msg = apiErr.Message
		}
		return nil, fmt.Errorf("bunny dns %s %s: %s", method, path, msg)
	}
	return data, nil
}
