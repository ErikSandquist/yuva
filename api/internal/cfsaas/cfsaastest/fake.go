// Package cfsaastest is a fake of the Cloudflare custom hostnames API for tests, answering in the
// documented shapes (https://developers.cloudflare.com/api/resources/custom_hostnames/).
package cfsaastest

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/productdevbook/yuva/api/internal/cfsaas"
)

type hostname struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	Status   string `json:"status"`
	SSL      struct {
		Method string `json:"method"`
		Type   string `json:"type"`
		Status string `json:"status"`
	} `json:"ssl"`
}

type Fake struct {
	zone, token string
	srv         *httptest.Server
	mu          sync.Mutex
	byID        map[string]*hostname
}

func New(t *testing.T, zone, token string) *Fake {
	f := &Fake{zone: zone, token: token, byID: map[string]*hostname{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *Fake) Client() *cfsaas.Client {
	return &cfsaas.Client{BaseURL: f.srv.URL + "/client/v4", ZoneID: f.zone, Token: f.token, HTTP: f.srv.Client()}
}

func (f *Fake) find(name string) *hostname {
	for _, h := range f.byID {
		if h.Hostname == name {
			return h
		}
	}
	return nil
}

func (f *Fake) Has(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.find(name) != nil
}

func (f *Fake) SSLMethod(name string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if h := f.find(name); h != nil {
		return h.SSL.Method
	}
	return ""
}

// Activate marks the hostname and its certificate active, as Cloudflare does once validation passed.
func (f *Fake) Activate(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if h := f.find(name); h != nil {
		h.Status, h.SSL.Status = "active", "active"
	}
}

func reply(w http.ResponseWriter, status int, result any, errs ...string) {
	type apiErr struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	body := map[string]any{"success": len(errs) == 0, "errors": []apiErr{}, "messages": []string{}, "result": result}
	for _, e := range errs {
		code, msg, _ := strings.Cut(e, " ")
		var n int
		_ = json.Unmarshal([]byte(code), &n)
		body["errors"] = append(body["errors"].([]apiErr), apiErr{Code: n, Message: msg})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+f.token {
		reply(w, http.StatusForbidden, nil, "10000 Authentication error")
		return
	}
	prefix := "/client/v4/zones/" + f.zone + "/custom_hostnames"
	rest, ok := strings.CutPrefix(r.URL.Path, prefix)
	if !ok {
		reply(w, http.StatusNotFound, nil, "7003 Could not route to "+r.URL.Path)
		return
	}
	id := strings.TrimPrefix(rest, "/")
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodPost && id == "":
		var in struct {
			Hostname string `json:"hostname"`
			SSL      struct {
				Method string `json:"method"`
				Type   string `json:"type"`
			} `json:"ssl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Hostname == "" {
			reply(w, http.StatusBadRequest, nil, "1400 Bad request")
			return
		}
		if f.find(in.Hostname) != nil {
			reply(w, http.StatusConflict, nil, "1406 Duplicate custom hostname found.")
			return
		}
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		x := hex.EncodeToString(b)
		h := &hostname{ID: x[:8] + "-" + x[8:12] + "-" + x[12:16] + "-" + x[16:20] + "-" + x[20:], Hostname: in.Hostname, Status: "pending"}
		h.SSL.Method, h.SSL.Type, h.SSL.Status = in.SSL.Method, in.SSL.Type, "pending_validation"
		f.byID[h.ID] = h
		reply(w, http.StatusOK, h)
	case r.Method == http.MethodGet && id == "":
		out := []*hostname{}
		for _, h := range f.byID {
			if q := r.URL.Query().Get("hostname.exact"); q == "" || q == h.Hostname {
				out = append(out, h)
			}
		}
		reply(w, http.StatusOK, out)
	case r.Method == http.MethodGet:
		if h, ok := f.byID[id]; ok {
			reply(w, http.StatusOK, h)
			return
		}
		reply(w, http.StatusNotFound, nil, "1436 The custom hostname was not found.")
	case r.Method == http.MethodDelete:
		if _, ok := f.byID[id]; !ok {
			reply(w, http.StatusNotFound, nil, "1436 The custom hostname was not found.")
			return
		}
		delete(f.byID, id)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
	default:
		reply(w, http.StatusMethodNotAllowed, nil, "10405 Method not allowed")
	}
}
