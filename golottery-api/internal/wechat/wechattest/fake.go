// Package wechattest is a stand-in WeChat server for tests and local runs.
package wechattest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"sync"
)

// QRRequest is one getwxacodeunlimit call as the fake received it.
type QRRequest struct {
	Token      string
	Scene      string `json:"scene"`
	Page       string `json:"page"`
	EnvVersion string `json:"env_version"`
	CheckPath  bool   `json:"check_path"`
}

// Server counts calls and can be told to fail.
type Server struct {
	*httptest.Server

	mu         sync.Mutex
	tokenCalls int
	qrCalls    []QRRequest
	// ExpiresIn is returned with every token.
	ExpiresIn int
	// RejectTokens makes getwxacodeunlimit answer 40001 for the next n calls.
	RejectTokens int
	// QRErrCode, when set, is returned by getwxacodeunlimit instead of an image.
	QRErrCode int
}

// New starts a fake server; close it with Close.
func New() *Server {
	s := &Server{ExpiresIn: 7200}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cgi-bin/stable_token", s.stableToken)
	mux.HandleFunc("POST /wxa/getwxacodeunlimit", s.qrcode)
	mux.HandleFunc("GET /sns/jscode2session", s.code2session)
	s.Server = httptest.NewServer(mux)
	return s
}

func (s *Server) stableToken(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	s.mu.Lock()
	s.tokenCalls++
	n, expires := s.tokenCalls, s.ExpiresIn
	s.mu.Unlock()
	if body["appid"] == "" || body["secret"] == "" || body["force_refresh"] != false {
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 40013, "errmsg": "invalid appid"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": fmt.Sprintf("token-%d", n), "expires_in": expires})
}

func (s *Server) qrcode(w http.ResponseWriter, r *http.Request) {
	var req QRRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	req.Token = r.URL.Query().Get("access_token")
	s.mu.Lock()
	s.qrCalls = append(s.qrCalls, req)
	reject := s.RejectTokens > 0
	if reject {
		s.RejectTokens--
	}
	errCode := s.QRErrCode
	s.mu.Unlock()
	switch {
	case reject:
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 40001, "errmsg": "invalid credential"})
	case errCode != 0:
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": errCode, "errmsg": "fake error"})
	default:
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(sampleJPEG())
	}
}

// code2session maps code "c-<name>" to openid "o-<name>"; any other code is invalid.
func (s *Server) code2session(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("js_code")
	if len(code) > 2 && code[:2] == "c-" {
		_ = json.NewEncoder(w).Encode(map[string]any{"openid": "o-" + code[2:], "session_key": "k"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 40029, "errmsg": "invalid code"})
}

// TokenCalls is how many times stable_token was called.
func (s *Server) TokenCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenCalls
}

// QRCalls returns the getwxacodeunlimit calls so far.
func (s *Server) QRCalls() []QRRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]QRRequest(nil), s.qrCalls...)
}

func sampleJPEG() []byte {
	img := image.NewGray(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = color.Gray{Y: uint8(i * 4)}.Y
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}
