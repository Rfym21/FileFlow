package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrUnsafeDownloadURL = errors.New("URL 上传只允许访问公网 HTTP 或 HTTPS 地址")

var nonPublicDownloadNetworks = func() []*net.IPNet {
	result := make([]*net.IPNet, 0)
	for _, cidr := range []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24",
		"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
		"64:ff9b::/96", "64:ff9b:1::/48", "2001::/32", "2001:2::/48",
		"2001:10::/28", "2001:20::/28", "2001:db8::/32", "2002::/16",
	} {
		_, network, _ := net.ParseCIDR(cidr)
		result = append(result, network)
	}
	return result
}()

func publicDownloadIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, network := range nonPublicDownloadNetworks {
		if network.Contains(ip) {
			return false
		}
	}
	return true
}

func validateDownloadURL(address *url.URL) error {
	if address == nil || (address.Scheme != "http" && address.Scheme != "https") || address.Hostname() == "" || address.User != nil {
		return ErrUnsafeDownloadURL
	}
	if ip := net.ParseIP(address.Hostname()); ip != nil && !publicDownloadIP(ip) {
		return ErrUnsafeDownloadURL
	}
	return nil
}

type downloadResolver func(context.Context, string) ([]net.IPAddr, error)
type downloadDialer func(context.Context, string, string) (net.Conn, error)

// Resolve, validate every result, and dial a pinned IP rather than re-resolving
// the hostname. This also prevents a redirect or DNS change from reaching LANs.
func publicDownloadDial(resolve downloadResolver, dial downloadDialer) downloadDialer {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, ErrUnsafeDownloadURL
		}
		ips := []net.IPAddr{}
		if ip := net.ParseIP(host); ip != nil {
			ips = append(ips, net.IPAddr{IP: ip})
		} else {
			ips, err = resolve(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("解析下载地址失败: %w", err)
			}
		}
		if len(ips) == 0 {
			return nil, ErrUnsafeDownloadURL
		}
		for _, ip := range ips {
			if ip.Zone != "" || !publicDownloadIP(ip.IP) {
				return nil, ErrUnsafeDownloadURL
			}
		}
		for _, ip := range ips {
			connection, dialErr := dial(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if dialErr == nil {
				return connection, nil
			}
			err = dialErr
		}
		return nil, err
	}
}

func publicDownloadRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("下载地址重定向次数过多")
	}
	return validateDownloadURL(request.URL)
}

// DownloadPublicURL fetches remote uploads without exposing server-local services.
// It deliberately ignores HTTP_PROXY/HTTPS_PROXY so network validation applies
// to the connection that carries the request.
func DownloadPublicURL(rawURL string) (*http.Response, error) {
	address, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || validateDownloadURL(address) != nil {
		return nil, ErrUnsafeDownloadURL
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	client := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           publicDownloadDial(net.DefaultResolver.LookupIPAddr, dialer.DialContext),
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			DisableKeepAlives:     true,
		},
		CheckRedirect: publicDownloadRedirect,
	}
	return client.Get(address.String())
}
