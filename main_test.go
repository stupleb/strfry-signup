package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var placeholder = regexp.MustCompile(`\{\w+\}`)

// the script's messages keep their placeholders for app.js to fill
var scriptStrings = regexp.MustCompile(`data-strings="[^"]*"`)

func TestLocalesComplete(t *testing.T) {
	for _, l := range locales[1:] {
		for key, en := range messagesEN {
			msg, ok := l.msgs[key]
			if !ok {
				t.Errorf("%s: missing %q", l.code, key)
				continue
			}
			want := slices.Sorted(slices.Values(placeholder.FindAllString(en, -1)))
			got := slices.Sorted(slices.Values(placeholder.FindAllString(msg, -1)))
			if !slices.Equal(want, got) {
				t.Errorf("%s %q: placeholders %v, want %v", l.code, key, got, want)
			}
		}
		for key := range l.msgs {
			if _, ok := messagesEN[key]; !ok {
				t.Errorf("%s: %q is not an English key", l.code, key)
			}
		}
	}
}

func TestNegotiate(t *testing.T) {
	for header, want := range map[string]string{
		"":                           "en",
		"*":                          "en",
		"ja":                         "en",
		"es-MX,es;q=0.9,en;q=0.8":    "es",
		"en-GB,de;q=0.9":             "en",
		"ja, pl;q=0.5, fr;q=0.7":     "fr",
		"de;q=0.2, fr ; q=0.1":       "de",
		"fr;q=0, de;q=0.1":           "de",
		"fr;q=bad, pl;q=0.3":         "pl",
		"fr_FR":                      "fr",
		"pt-BR,pt;q=0.9,es-ES;q=0.4": "es",
	} {
		if got := negotiate(header).code; got != want {
			t.Errorf("negotiate(%q) = %s, want %s", header, got, want)
		}
	}
}

func TestFillsPlaceholdersOnce(t *testing.T) {
	got := locales[0].t("memo", "relay", "{key}", "key", "npub1x")
	if want := "Write access to {key} for npub1x"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNumber(t *testing.T) {
	for _, c := range []struct {
		code string
		n    int64
		want string
	}{
		{"en", 999, "999"},
		{"en", 2100, "2,100"},
		{"en", 1000000, "1,000,000"},
		{"es", 2100, "2100"},
		{"es", 21000, "21.000"},
		{"de", 2100, "2.100"},
		{"pl", 2100, "2100"},
		{"pl", 21000, "21 000"},
		{"fr", 2100, "2 100"},
	} {
		if got := findLocale(c.code).number(c.n); got != c.want {
			t.Errorf("%s number(%d) = %q, want %q", c.code, c.n, got, c.want)
		}
	}
}

func TestPages(t *testing.T) {
	pages, err := parsePages()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range locales {
		for _, c := range []struct {
			accepting bool
			backend   backend
			period    int64
		}{{false, lnbits{}, 30}, {true, nil, 30}, {true, lnbits{}, 0}, {true, lnbits{}, 1}, {true, lnbits{}, 30}} {
			s := &server{
				accepting: c.accepting, backend: c.backend, priceSats: 21000, periodDays: c.period, pages: pages,
				relay: relayInfo{Name: "Relay", URL: "wss://relay.example", Contact: "a@b.example"},
			}
			w := httptest.NewRecorder()
			s.index(w, httptest.NewRequest("GET", "/?lang="+l.code, nil))
			page := w.Body.String()
			if left := placeholder.FindString(scriptStrings.ReplaceAllString(page, "")); left != "" {
				t.Errorf("%s %+v: unfilled %s", l.code, c, left)
			}
			if !strings.Contains(page, `<html lang="`+l.tag+`">`) || w.Header().Get("Content-Language") != l.tag {
				t.Errorf("%s %+v: not served as %s", l.code, c, l.tag)
			}
			if !strings.Contains(page, `<span aria-current="true" lang="`+l.tag+`">`+l.name+`</span>`) {
				t.Errorf("%s %+v: language menu does not mark %s", l.code, c, l.name)
			}
		}
	}
}

func TestPubkeys(t *testing.T) {
	const hexKey = "7e7e9c42a91bfef19fa929e5fda1b72e0ebc1a4c1141673e2794234d86addf4e"
	const npub = "npub10elfcs4fr0l0r8af98jlmgdh9c8tcxjvz9qkw038js35mp4dma8qzvjptg"
	if got := encodeNpub(hexKey); got != npub {
		t.Errorf("encodeNpub = %s", got)
	}
	for _, in := range []string{npub, strings.ToUpper(hexKey), " " + npub + " "} {
		if got, ok := parsePubkey(in); !ok || got != hexKey {
			t.Errorf("parsePubkey(%q) = %q, %v", in, got, ok)
		}
	}
	for _, in := range []string{npub[:len(npub)-1] + "q", hexKey[:63], "nsec1" + npub[5:]} {
		if _, ok := parsePubkey(in); ok {
			t.Errorf("parsePubkey(%q) accepted", in)
		}
	}
}

func TestRateKey(t *testing.T) {
	for ip, want := range map[string]string{
		"203.0.113.7":            "203.0.113.7",
		"2001:db8:1:2ff:3:4:5:6": "2001:db8:1:200::",
		"2001:db8:1:200::1":      "2001:db8:1:200::",
		"2001:db8:1:300::1":      "2001:db8:1:300::",
		"::ffff:203.0.113.7":     "::ffff:203.0.113.7",
		"fe80::a:b:c:d":          "fe80::a:b:c:d",
		"fd12:3456:789a:1::5":    "fd12:3456:789a:1::5",
	} {
		if got := rateKey(ip); got != want {
			t.Errorf("rateKey(%s) = %s, want %s", ip, got, want)
		}
	}
}

func TestClientIP(t *testing.T) {
	s := &server{trusted: []netip.Addr{netip.MustParseAddr("10.0.3.1")}}
	for _, c := range []struct {
		remote string
		xff    []string
		want   string
	}{
		{"10.0.3.1:5000", []string{"203.0.113.7"}, "203.0.113.7"},
		{"10.0.3.1:5000", []string{"198.51.100.1, 203.0.113.7"}, "203.0.113.7"},
		{"10.0.3.1:5000", []string{"198.51.100.1", "203.0.113.7"}, "203.0.113.7"},
		{"[::ffff:10.0.3.1]:5000", []string{"2001:db8::1"}, "2001:db8::1"},
		{"10.0.3.1:5000", nil, "10.0.3.1"},
		{"10.0.3.2:5000", []string{"203.0.113.7"}, "10.0.3.2"},
		{"[2001:db8::5]:5000", []string{"203.0.113.7"}, "2001:db8::5"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remote
		r.Header["X-Forwarded-For"] = c.xff
		if got := s.clientIP(r); got != c.want {
			t.Errorf("%s with X-Forwarded-For %q: %s, want %s", c.remote, c.xff, got, c.want)
		}
	}
}

type fakeBackend struct{ created atomic.Int64 }

func (f *fakeBackend) create(context.Context, int64, string, string) (newInvoice, error) {
	return newInvoice{id: fmt.Sprint("inv", f.created.Add(1)), bolt11: "lnbc1", expiresAt: time.Now().Add(invoiceLifetime).Unix()}, nil
}
func (*fakeBackend) status(context.Context, string) (payState, error) { return statePending, nil }
func (*fakeBackend) check(context.Context) error                      { return nil }

func TestOpenInvoicesPerClient(t *testing.T) {
	st, err := loadState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &server{
		accepting: true, backend: &fakeBackend{}, priceSats: 21000, periodDays: 30, st: st,
		trusted: []netip.Addr{netip.MustParseAddr("10.0.3.1")},
		byIP:    map[string][]time.Time{}, creating: map[string]int{},
	}
	h := s.handler(nil)
	key := func(n int) string { return fmt.Sprintf("%064x", n) }
	ask := func(client string, n int) int {
		r := httptest.NewRequest("POST", "/join/api/invoice", strings.NewReader(`{"pubkey":"`+key(n)+`"}`))
		r.RemoteAddr = "10.0.3.1:5000"
		r.Header.Set("X-Forwarded-For", client)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	for n, client := range []string{"2001:db8:1:2ff::1", "2001:db8:1:200::2", "2001:db8:1:2aa::3"} {
		if code := ask(client, n+1); code != http.StatusCreated {
			t.Fatalf("invoice %d: %d", n+1, code)
		}
	}
	for _, c := range []struct {
		client string
		key    int
		want   int
	}{
		{"2001:db8:1:2bb::4", 4, http.StatusTooManyRequests},
		{"2001:db8:1:2ff::1", 1, http.StatusOK},
		{"2001:db8:1:300::1", 4, http.StatusCreated},
	} {
		if code := ask(c.client, c.key); code != c.want {
			t.Errorf("key %d from %s: %d, want %d", c.key, c.client, code, c.want)
		}
	}
	st.mu.Lock()
	for _, inv := range st.Invoices {
		if inv.Pubkey == key(2) {
			inv.Status = statePaid
		}
	}
	st.mu.Unlock()
	if code := ask("2001:db8:1:2ff::1", 5); code != http.StatusCreated {
		t.Errorf("once one is paid: %d", code)
	}
	if len(s.creating) != 0 {
		t.Errorf("reservations left: %v", s.creating)
	}
}

func TestCheckInterval(t *testing.T) {
	now := time.Now()
	open := now.Add(10 * time.Minute).Unix()
	for _, c := range []struct {
		inv  invoice
		want time.Duration
	}{
		{invoice{Status: statePending, ExpiresAt: open, pageSeen: now.Add(-5 * time.Second)}, 3 * time.Second},
		{invoice{Status: statePending, ExpiresAt: open, pageSeen: now.Add(-time.Minute)}, 30 * time.Second},
		{invoice{Status: statePending, ExpiresAt: now.Unix() - 1, pageSeen: now}, time.Minute},
		{invoice{Status: stateProcessing, ExpiresAt: open, pageSeen: now}, time.Minute},
	} {
		if got := checkInterval(&c.inv, now); got != c.want {
			t.Errorf("%s invoice seen %v ago: %v, want %v", c.inv.Status, now.Sub(c.inv.pageSeen), got, c.want)
		}
	}

	inv := &invoice{Status: statePending, ExpiresAt: open, nextCheck: now.Add(30 * time.Second)}
	s := &server{st: &state{stateFile: stateFile{Invoices: map[string]*invoice{"x": inv}}}}
	w := httptest.NewRecorder()
	s.handler(nil).ServeHTTP(w, httptest.NewRequest("GET", "/join/api/invoice/x", nil))
	if w.Code != http.StatusOK || inv.nextCheck.After(time.Now().Add(3*time.Second)) {
		t.Errorf("after the page asked: %d, next check in %v", w.Code, time.Until(inv.nextCheck))
	}
}

func TestPrune(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	ago := func(d time.Duration) int64 { return now.Add(-d).Unix() }
	s := &state{stateFile: stateFile{Invoices: map[string]*invoice{
		"paid":             {Status: statePaid, ExpiresAt: ago(2 * time.Hour)},
		"paid long ago":    {Status: statePaid, ExpiresAt: ago(25 * time.Hour)},
		"expired":          {Status: stateExpired, ExpiresAt: ago(50 * time.Minute)},
		"expired long ago": {Status: stateExpired, ExpiresAt: ago(70 * time.Minute)},
		"still checked":    {Status: statePending, ExpiresAt: ago(23 * time.Hour)},
		"confirming":       {Status: stateProcessing, ExpiresAt: ago(30 * time.Hour)},
	}}}
	if !s.prune(now) {
		t.Error("pruned nothing")
	}
	if got := slices.Sorted(maps.Keys(s.Invoices)); !slices.Equal(got, []string{"confirming", "expired", "paid", "still checked"}) {
		t.Errorf("kept %v", got)
	}
}

func TestCredit(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	day := int64(86400)
	ptr := func(n int64) *int64 { return &n }
	s := &state{stateFile: stateFile{Members: map[string]member{}}}

	s.credit(&invoice{Pubkey: "a"}, 30, now)
	if got := *s.Members["a"].Until; got != now.Unix()+30*day {
		t.Errorf("new member until %d", got)
	}
	s.credit(&invoice{Pubkey: "a", PeriodDays: ptr(10)}, 30, now)
	if got := *s.Members["a"].Until; got != now.Unix()+40*day {
		t.Errorf("renewal stacks to %d, want the invoice's own period added", got)
	}
	s.Members["b"] = member{Until: ptr(now.Unix() - day), Since: 1}
	s.credit(&invoice{Pubkey: "b", PeriodDays: ptr(30)}, 30, now)
	if m := s.Members["b"]; *m.Until != now.Unix()+30*day || m.Since != 1 {
		t.Errorf("lapsed member %+v", m)
	}
	s.credit(&invoice{Pubkey: "c", PeriodDays: ptr(0)}, 30, now)
	s.credit(&invoice{Pubkey: "c", PeriodDays: ptr(30)}, 30, now)
	if s.Members["c"].Until != nil {
		t.Error("permanent access became timed")
	}
	if !slices.Equal(slices.Sorted(maps.Keys(s.Members)), []string{"a", "b", "c"}) {
		t.Errorf("members %v", slices.Sorted(maps.Keys(s.Members)))
	}
}

func TestRouting(t *testing.T) {
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "relay %s %s host=%s xff=%s", r.Method, r.URL.Path, r.Host, r.Header.Get("X-Forwarded-For"))
	}))
	defer relay.Close()
	pages, err := parsePages()
	if err != nil {
		t.Fatal(err)
	}
	s := &server{
		accepting: true, backend: lnbits{}, priceSats: 21000, periodDays: 30, pages: pages, st: &state{stateFile: stateFile{Members: map[string]member{}}},
		trusted: []netip.Addr{netip.MustParseAddr("192.0.2.1")},
	}
	upstream, _ := url.Parse(relay.URL)
	const html, nip11 = "text/html,application/xhtml+xml", "application/nostr+json"
	for _, c := range []struct {
		method, path, accept string
		relay                bool
		code                 int
		want                 string
	}{
		{"GET", "/", html, true, http.StatusFound, "Location: /join/"},
		{"GET", "/?lang=es", html, true, http.StatusFound, "Location: /join/?lang=es"},
		{"GET", "/", nip11, true, http.StatusOK, "relay GET / host=relay.example xff=203.0.113.7"},
		{"POST", "/", html, true, http.StatusOK, "relay POST /"},
		{"GET", "/metrics", html, true, http.StatusOK, "relay GET /metrics"},
		{"GET", "/join", html, true, http.StatusTemporaryRedirect, "Location: /join/"},
		{"GET", "/join/", html, true, http.StatusOK, `<html lang="en-US">`},
		{"GET", "/join/app.js", "*/*", true, http.StatusOK, "async function api"},
		{"GET", "/join/api/member/7e7e9c42a91bfef19fa929e5fda1b72e0ebc1a4c1141673e2794234d86addf4e", "*/*", true, http.StatusOK, `"member":false`},
		{"GET", "/", html, false, http.StatusFound, "Location: /join/"},
		{"GET", "/", nip11, false, http.StatusNotFound, ""},
		{"GET", "/metrics", html, false, http.StatusNotFound, ""},
	} {
		var up *url.URL
		if c.relay {
			up = upstream
		}
		r := httptest.NewRequest(c.method, "http://relay.example"+c.path, nil)
		r.Header.Set("Accept", c.accept)
		r.Header.Set("X-Forwarded-For", "203.0.113.7")
		w := httptest.NewRecorder()
		s.handler(up).ServeHTTP(w, r)
		var got strings.Builder
		w.Result().Header.Write(&got)
		got.WriteString(w.Body.String())
		if w.Code != c.code || !strings.Contains(got.String(), c.want) {
			t.Errorf("%s %s (relay %v): %d %q, want %d containing %q", c.method, c.path, c.relay, w.Code, got.String(), c.code, c.want)
		}
		if proxied := strings.HasPrefix(w.Body.String(), "relay "); proxied && w.Header().Get("Content-Security-Policy") != "" {
			t.Errorf("%s %s: the page's headers reached a relay response", c.method, c.path)
		}
	}

	r := httptest.NewRequest("GET", "http://relay.example/", nil)
	r.RemoteAddr = "198.51.100.9:1234"
	r.Header.Set("Accept", nip11)
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	w := httptest.NewRecorder()
	s.handler(upstream).ServeHTTP(w, r)
	if !strings.HasSuffix(w.Body.String(), "xff=198.51.100.9") {
		t.Errorf("from an untrusted address: %q", w.Body.String())
	}
}

// A proxied websocket must outlive the server's read and write timeouts.
func TestRelayWebsocketOutlivesTimeouts(t *testing.T) {
	gotXFF := make(chan string, 1)
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXFF <- r.Header.Get("X-Forwarded-For")
		conn, brw, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n"))
		io.Copy(conn, brw)
	}))
	defer relay.Close()
	upstream, _ := url.Parse(relay.URL)
	front := httptest.NewUnstartedServer((&server{trusted: []netip.Addr{netip.MustParseAddr("127.0.0.1")}}).handler(upstream))
	front.Config.ReadTimeout = 100 * time.Millisecond
	front.Config.WriteTimeout = 100 * time.Millisecond
	front.Start()
	defer front.Close()

	conn, err := net.Dial("tcp", front.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprint(conn, "GET / HTTP/1.1\r\nHost: relay.example\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nX-Forwarded-For: 203.0.113.7\r\n\r\n")
	br := bufio.NewReader(conn)
	res, err := http.ReadResponse(br, nil)
	if err != nil || res.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("handshake: %v %v", res, err)
	}
	if xff := <-gotXFF; xff != "203.0.113.7" {
		t.Errorf("relay saw X-Forwarded-For %q", xff)
	}
	time.Sleep(300 * time.Millisecond)
	fmt.Fprint(conn, "ping\n")
	if line, err := br.ReadString('\n'); line != "ping\n" {
		t.Errorf("after the timeouts: %q, %v", line, err)
	}
}
