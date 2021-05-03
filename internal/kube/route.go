package kube

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"regexp"
	"strconv"
	"strings"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
)

type (
	// Route is an openshift route
	Route struct {
		*routev1.Route
		ClusterName string
	}

	// ProbeInfo
	ProbeInfo struct {
		Skip bool

		SSL   bool
		Host  string
		Proto string
		Path  string
		URL   string

		Method           string
		ValidStatusCodes []string
		BodyRegex        string

		Cluster string
		UID     string
	}

	// RequestMetrics
	RequestMetrics struct {
		*ProbeInfo

		Start         time.Time
		Resolved      time.Duration
		Connected     time.Duration
		WroteRequest  time.Duration
		ReadFirstByte time.Duration
		ReadBody      time.Duration
		Expires       time.Time
		Size          int64
		RedirectCount int64

		InvalidRouteErr      bool
		InvalidRequestErr    bool
		ConnectionErr        bool
		BodyDownloadErr      bool
		InvalidStatusCodeErr bool
		InvalidBodyRegexErr  bool
		InvalidBodyErr       bool
	}
)

func (r *Route) getProbeInfo() *ProbeInfo {
	host := r.Spec.Host
	ssl := r.Spec.TLS != nil
	proto := "https"
	if !ssl {
		proto = "http"
	}
	path := r.Spec.Path
	url := fmt.Sprintf("%s://%s", proto, host)
	skip := false
	if strings.HasPrefix(path, "/.well-known/acme-challenge/") {
		skip = true
	}
	if as, ok := r.GetAnnotations()["thobits.com/ormon-skip"]; ok {
		skip, _ = strconv.ParseBool(as)
	}
	method := "GET"
	if am, ok := r.GetAnnotations()["thobits.com/ormon-method"]; ok {
		method = strings.ToUpper(am)
	}
	validStatusCodes := []string{"200"}
	if avsc, ok := r.GetAnnotations()["thobits.com/ormon-valid-statuscodes"]; ok {
		validStatusCodes = strings.Split(avsc, ",")
	}
	bodyRegex := ""
	if abr, ok := r.GetAnnotations()["thobits.com/ormon-body-regex"]; ok {
		bodyRegex = abr
	}

	return &ProbeInfo{
		Skip: skip,

		Host:  host,
		Proto: proto,
		Path:  r.Spec.Path,
		URL:   url,

		Method:           method,
		ValidStatusCodes: validStatusCodes,
		BodyRegex:        bodyRegex,

		SSL:     ssl,
		Cluster: r.ClusterName,
		UID:     string(r.GetUID()),
	}
}

// Probe gathers the metrics for a route
func (r *Route) Probe(ctx context.Context) (m *RequestMetrics) {
	// prepare
	pi := r.getProbeInfo()
	m = &RequestMetrics{ProbeInfo: pi}
	if m.Skip {
		return nil
	}
	req, err := http.NewRequest(m.Method, m.URL, nil)
	if err != nil {
		m.InvalidRequestErr = true
		logrus.Errorf("%s %s %s", err, m.Cluster, m.Host)
		return
	}
	req = req.WithContext(ctx)

	// metrics storage
	trace := &httptrace.ClientTrace{
		DNSStart: func(dsi httptrace.DNSStartInfo) {
			m.Start = time.Now()
		},
		DNSDone: func(ddi httptrace.DNSDoneInfo) {
			m.Resolved = time.Since(m.Start)
		},
		ConnectDone: func(network, addr string, err error) {
			m.Connected = time.Since(m.Start)
		},
		WroteRequest: func(wri httptrace.WroteRequestInfo) {
			m.WroteRequest = time.Since(m.Start)
		},
		GotFirstResponseByte: func() {
			m.ReadFirstByte = time.Since(m.Start)
		},
	}

	// request
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := http.Client{
		Transport: http.DefaultTransport,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			redirects := len(via)
			m.RedirectCount = int64(redirects)
			if redirects > 10 {
				return fmt.Errorf("to many redirects (%d)", redirects)
			}
			return nil
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	resp, err := client.Do(req)
	if err != nil {
		m.ConnectionErr = true
		logrus.Errorf("%s %s %s", err, m.Cluster, m.Host)
		return
	}

	// statuscode
	statusCodeIsValid := false
	for _, sc := range m.ValidStatusCodes {
		statusCodeIsValid = statusCodeIsValid || (sc == strconv.Itoa(resp.StatusCode))
	}
	if !statusCodeIsValid {
		m.InvalidStatusCodeErr = true
		msg := fmt.Sprintf("statuscode %d not in %s", resp.StatusCode, strings.Join(m.ValidStatusCodes, ","))
		logrus.Errorf("%s %s %s", msg, m.Cluster, m.Host)
	}

	// read body
	body := bytes.NewBufferString("")
	respBodyReader := newCtxReader(ctx, resp.Body)
	m.Size, err = io.Copy(body, respBodyReader)
	if err != nil {
		m.BodyDownloadErr = true
		logrus.Errorf("%s %s %s", err, m.Cluster, m.Host)
		return
	}
	m.ReadBody = time.Since(m.Start)
	bodyRegex, err := regexp.Compile(m.BodyRegex)
	if err != nil {
		m.InvalidBodyRegexErr = true
		logrus.Errorf("%s %s %s", err, m.Cluster, m.Host)
		return
	}
	match := bodyRegex.FindReaderIndex(body)
	if match == nil {
		m.InvalidBodyErr = true
		logrus.Errorf("%s %s %s", "body regex does not match", m.Cluster, m.Host)
		return
	}

	// ssl
	m.Expires = time.Time{}
	if resp.TLS != nil {
		m.Expires = expiresFirst(resp.TLS.PeerCertificates)
	}

	return m
}

func expiresFirst(certs []*x509.Certificate) time.Time {
	earliest := time.Time{}
	for _, c := range certs {
		if (earliest.IsZero() || c.NotAfter.Before(earliest)) && !c.NotAfter.IsZero() {
			earliest = c.NotAfter
		}
	}
	return earliest
}
