// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Иргэний PKI самбар endpoint-уудын (summary/certificates/devices/activity)
// client unit тест: wire задлалт, 403 → ErrPKINotPermitted, path + query.
package eid

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPersonCertificates(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/certificates/etsi/PNOMN-УБ1") {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"personEtsi":"PNOMN-УБ1","counts":{"valid":2,"revoked":1,"expired":0,"suspended":0,"total":3},
			"certificates":[{"documentNumber":"D1","type":"AUTH","serialNumber":"1a","certificateLevel":"QUALIFIED","status":"VALID","issuerDn":"CN=CA"}]}`)
	})
	res, err := c.PersonCertificates(context.Background(), "PNOMN-УБ1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts.Valid != 2 || res.Counts.Revoked != 1 || res.Counts.Total != 3 {
		t.Errorf("counts = %+v", res.Counts)
	}
	if len(res.Certificates) != 1 || res.Certificates[0].Type != "AUTH" || res.Certificates[0].Status != "VALID" {
		t.Errorf("certs = %+v", res.Certificates)
	}
}

func TestPersonDevices(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"devices":[{"documentNumber":"D1","platform":"APNS","active":true}],"activeCount":1,"total":2}`)
	})
	res, err := c.PersonDevices(context.Background(), "PNOMN-УБ1")
	if err != nil {
		t.Fatal(err)
	}
	if res.ActiveCount != 1 || res.Total != 2 || len(res.Devices) != 1 || !res.Devices[0].Active {
		t.Errorf("devices = %+v", res)
	}
}

func TestPersonActivity(t *testing.T) {
	var gotQuery string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"counts":{"authentication":5,"signature":2},
			"sessions":[{"sessionId":"s1","flow":"AUTHENTICATION","outcome":"OK"}],"total":7}`)
	})
	res, err := c.PersonActivity(context.Background(), "PNOMN-УБ1", 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "limit=10&offset=5" {
		t.Errorf("query = %s", gotQuery)
	}
	if res.Counts.Authentication != 5 || res.Counts.Signature != 2 || res.Total != 7 {
		t.Errorf("activity = %+v", res)
	}
}

func TestPersonSummary(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"givenName":"Бат","certificates":{"valid":2,"total":3},
			"activity":{"authentication":5,"signature":1},"devicesActive":1,"devicesTotal":2,"representationCount":1}`)
	})
	res, err := c.PersonSummary(context.Background(), "PNOMN-УБ1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Certificates.Valid != 2 || res.Activity.Authentication != 5 || res.DevicesActive != 1 || res.RepresentationCount != 1 {
		t.Errorf("summary = %+v", res)
	}
}

func TestPKIForbiddenMapsToSentinel(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"RP-д PKI_READ эрх алга"}`)
	})
	for _, call := range []func() error{
		func() error { _, e := c.PersonSummary(context.Background(), "PNOMN-X"); return e },
		func() error { _, e := c.PersonCertificates(context.Background(), "PNOMN-X"); return e },
		func() error { _, e := c.PersonDevices(context.Background(), "PNOMN-X"); return e },
		func() error { _, e := c.PersonActivity(context.Background(), "PNOMN-X", 20, 0); return e },
	} {
		if err := call(); !errors.Is(err, ErrPKINotPermitted) {
			t.Errorf("403 → ErrPKINotPermitted хүлээсэн, авсан %v", err)
		}
	}
}

func TestPKIEmptyPersonEtsiErrors(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
	if _, err := c.PersonSummary(context.Background(), " "); err == nil {
		t.Error("хоосон personEtsi → алдаа")
	}
}
