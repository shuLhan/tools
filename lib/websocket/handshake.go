// Copyright 2018, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bytes"
	"errors"
	"net/http"
	"sync"
)

// List of handshake headers that must appear only once.
const (
	_hdrFlagHost int = 1 << iota
	_hdrFlagConn
	_hdrFlagUpgrade
	_hdrFlagWSKey
	_hdrFlagWSVersion
	_hdrFlagWSExtensions
	_hdrFlagWSProtocol
)

// List of errors.
var (
	ErrBadRequest                = errors.New("Bad request")
	ErrRequestLength             = errors.New("Bad request: length is less than minimum")
	ErrRequestHeaderLength       = errors.New("Bad request: header length is less than minimum")
	ErrInvalidHTTPMethod         = errors.New("Invalid HTTP method")
	ErrInvalidHTTPVersion        = errors.New("Invalid HTTP version")
	ErrInvalidHeaderUpgrade      = errors.New("Invalid Upgrade header")
	ErrInvalidHeaderFormat       = errors.New("Invalid Header format")
	ErrInvalidHeaderHost         = errors.New("Invalid Host header")
	ErrInvalidHeaderWSKey        = errors.New("Invalid Sec-Websocket-Key header")
	ErrInvalidHeaderWSVersion    = errors.New("Invalid Sec-Websocket-Version header")
	ErrInvalidHeaderWSExtensions = errors.New("Invalid Sec-Websocket-Extensions header")
	ErrInvalidHeaderWSProtocol   = errors.New("Invalid Sec-Websocket-Protocol header")
	ErrInvalidHeaderConn         = errors.New("Invalid Connection header")
	ErrMissingRequiredHeader     = errors.New("Missing required headers")
	ErrUnsupportedWSVersion      = errors.New("Unsupported Sec-WebSocket-Version")
)

//
//	RFC6455 4.1 P20
//	Please note that according to [RFC2616], all header field names in
//	both HTTP requests and HTTP responses are case-insensitive.
//
var (
	_hdrKeyConnection        = "connection"
	_hdrKeyHost              = "host"
	_hdrKeyOrigin            = "origin"
	_hdrKeyUpgrade           = "upgrade"
	_hdrKeyWSAccept          = "sec-websocket-accept"
	_hdrKeyWSExtensions      = "sec-websocket-extensions"
	_hdrKeyWSKey             = "sec-websocket-key"
	_hdrKeyWSProtocol        = "sec-websocket-protocol"
	_hdrKeyWSVersion         = "sec-websocket-version"
	_hdrValUpgradeWS         = "websocket"
	_hdrValConnectionUpgrade = "upgrade"
	_hdrValWSVersion         = "13"
	_bGET                    = []byte("GET")
	_bHTTP                   = []byte("HTTP")
	_bV11                    = []byte("1.1")
)

var (
	_handshakePool = sync.Pool{
		New: func() interface{} {
			return new(Handshake)
		},
	}
)

//
// Handshake contains the websocket HTTP handshake request.
//
type Handshake struct {
	start       int
	end         int
	headerFlags int
	raw         []byte

	URI        []byte
	Host       []byte
	Key        []byte
	Extensions []byte
	Protocol   []byte
	Header     http.Header
}

//
// Reset all handshake values to zero or empty.
//
func (h *Handshake) Reset(req []byte) {
	h.start = 0
	h.end = 0
	h.headerFlags = 0
	h.raw = req

	h.URI = nil
	h.Extensions = nil
	h.Protocol = nil
	h.Header = nil
}

func (h *Handshake) getBytesChunk(sep byte, tolower bool) (chunk []byte) {
	cr := false
	for h.end = h.start; h.end < len(h.raw); h.end++ {
		if h.raw[h.end] != sep {
			if h.raw[h.end] == '\r' {
				if h.start == h.end {
					return
				}
				cr = true
			} else if tolower {
				if h.raw[h.end] >= 'A' && h.raw[h.end] <= 'Z' {
					h.raw[h.end] += 32
				}
			}
			continue
		}
		break
	}

	if cr {
		chunk = h.raw[h.start : h.end-1]
	} else {
		chunk = h.raw[h.start:h.end]
	}
	h.start = h.end + 1

	return
}

//
// parseHTTPLine check if HTTP method is "GET", save the URI, and make sure
// that HTTP version is 1.1.
//
func (h *Handshake) parseHTTPLine() (err error) {
	chunk := h.getBytesChunk(' ', false)
	if !bytes.Equal(chunk, _bGET) {
		err = ErrInvalidHTTPMethod
		return
	}

	chunk = h.getBytesChunk(' ', false)
	if len(chunk) == 0 {
		err = ErrBadRequest
		return
	}

	h.URI = chunk

	chunk = h.getBytesChunk('/', false)
	if !bytes.Equal(chunk, _bHTTP) {
		err = ErrBadRequest
		return
	}

	chunk = h.getBytesChunk('\n', false)
	if !bytes.Equal(chunk, _bV11) {
		err = ErrInvalidHTTPVersion
		return
	}

	return
}

//
// parseHeader of HTTP request.
//
func (h *Handshake) parseHeader() (k, v []byte, err error) {
	chunk := h.getBytesChunk(':', true)
	if len(chunk) == 0 {
		return
	}
	if h.raw[h.start] != ' ' {
		err = ErrInvalidHeaderFormat
		return
	}
	h.start++

	k = chunk

	chunk = h.getBytesChunk('\n', false)
	if len(chunk) == 0 {
		err = ErrInvalidHeaderFormat
		return
	}

	v = chunk

	return
}

func (h *Handshake) headerValueContains(hv, sub []byte) bool {
	start := 0
	x := 0
	for ; x < len(hv); x++ {
		if hv[x] != ',' {
			if hv[x] == ' ' {
				start++
			} else {
				if hv[x] >= 'A' && hv[x] <= 'Z' {
					hv[x] += 32
				}
			}
			continue
		}
		if bytes.Equal(hv[start:x], sub) {
			return true
		}
		start = x + 1
	}

	return bytes.Equal(hv[start:], sub)
}

//
// Parse HTTP request.
//
//	RFC6455 4.1-P17-19
//
//	1.   The handshake MUST be a valid HTTP request as specified by
//	     [RFC2616].
//
//	2.   The method of the request MUST be GET, and the HTTP version MUST
//	     be at least 1.1.
//
//	     For example, if the WebSocket URI is "ws://example.com/chat",
//	     the first line sent should be "GET /chat HTTP/1.1".
//
//	3.   The "Request-URI" part of the request MUST match the /resource
//	     name/ defined in Section 3 (a relative URI) or be an absolute
//	     http/https URI that, when parsed, has a /resource name/, /host/,
//	     and /port/ that match the corresponding ws/wss URI.
//
//	4.   The request MUST contain a |Host| header field whose value
//	     contains /host/ plus optionally ":" followed by /port/ (when not
//	     using the default port).
//
//	5.   The request MUST contain an |Upgrade| header field whose value
//	     MUST include the "websocket" keyword.
//
//	6.   The request MUST contain a |Connection| header field whose value
//	     MUST include the "Upgrade" token.
//
//	7.   The request MUST include a header field with the name
//	     |Sec-WebSocket-Key|.  The value of this header field MUST be a
//	     nonce consisting of a randomly selected 16-byte value that has
//	     been base64-encoded (see Section 4 of [RFC4648]).  The nonce
//	     MUST be selected randomly for each connection.
//
//	     NOTE: As an example, if the randomly selected value was the
//	     sequence of bytes 0x01 0x02 0x03 0x04 0x05 0x06 0x07 0x08 0x09
//	     0x0a 0x0b 0x0c 0x0d 0x0e 0x0f 0x10, the value of the header
//	     field would be "AQIDBAUGBwgJCgsMDQ4PEC=="
//
//	     ...
//	     The |Sec-WebSocket-Key| header field MUST NOT appear more than once
//	     in an HTTP request.
//
//	8.   The request MUST include a header field with the name |Origin|
//	     [RFC6454] if the request is coming from a browser client.  If
//	     the connection is from a non-browser client, the request MAY
//	     include this header field if the semantics of that client match
//	     the use-case described here for browser clients.  The value of
//	     this header field is the ASCII serialization of origin of the
//	     context in which the code establishing the connection is
//	     running.  See [RFC6454] for the details of how this header field
//	     value is constructed.
//
//	     As an example, if code downloaded from www.example.com attempts
//	     to establish a connection to ww2.example.com, the value of the
//	     header field would be "http://www.example.com".
//
//	9.   The request MUST include a header field with the name
//	     |Sec-WebSocket-Version|.  The value of this header field MUST be
//	     13.
//
//	     NOTE: Although draft versions of this document (-09, -10, -11,
//	     and -12) were posted (they were mostly comprised of editorial
//	     changes and clarifications and not changes to the wire
//	     protocol), values 9, 10, 11, and 12 were not used as valid
//	     values for Sec-WebSocket-Version.  These values were reserved in
//	     the IANA registry but were not and will not be used.
//
//	10.  The request MAY include a header field with the name
//	     |Sec-WebSocket-Protocol|.  If present, this value indicates one
//	     or more comma-separated subprotocol the client wishes to speak,
//	     ordered by preference.  The elements that comprise this value
//	     MUST be non-empty strings with characters in the range U+0021 to
//	     U+007E not including separator characters as defined in
//	     [RFC2616] and MUST all be unique strings.  The ABNF for the
//	     value of this header field is 1#token, where the definitions of
//	     constructs and rules are as given in [RFC2616].
//
//	11.  The request MAY include a header field with the name
//	     |Sec-WebSocket-Extensions|.  If present, this value indicates
//	     the protocol-level extension(s) the client wishes to speak.  The
//	     interpretation and format of this header field is described in
//	     Section 9.1.
//
//	12.  The request MAY include any other header fields, for example,
//	     cookies [RFC6265] and/or authentication-related header fields
//	     such as the |Authorization| header field [RFC2616], which are
//	     processed according to documents that define them.
//
// Based on above requirements, the minimum handshake header is,
//
//	GET / HTTP/1.1\r\n			(16 bytes)
//	Host: a.com\r\n				(13 bytes)
//	Upgrade: websocket\r\n			(20 bytes)
//	Connection: Upgrade\r\n			(21 bytes)
//	Sec-Websocket-Key: (24 chars)\r\n	(45 bytes)
//	Sec-Websocket-Version: 13\r\n		(27 bytes)
//	\r\n					(2 bytes)
//
// If we count all characters, the minimum bytes would be: 144 bytes.  Of
// course one can send request with long URI "/chat?query=<512 chars>" without
// headers and the length will be greater than 144 bytes.
//
// The minimum length of request without HTTP line is: 144 - 16 = 128 bytes.
//
func (h *Handshake) Parse(req []byte) (err error) {
	if len(req) < 144 {
		err = ErrRequestLength
		return
	}

	h.Reset(req)

	err = h.parseHTTPLine()
	if err != nil {
		return
	}

	if len(h.raw)-h.start < 128 {
		err = ErrRequestHeaderLength
		return
	}

	var (
		k, v []byte
	)

	for h.start < len(h.raw) {
		k, v, err = h.parseHeader()
		if err != nil {
			return
		}
		if len(k) == 0 {
			break
		}

		switch {
		case bytes.Equal(k, []byte(_hdrKeyHost)):
			if h.headerFlags&_hdrFlagHost == _hdrFlagHost {
				err = ErrInvalidHeaderHost
				return
			}
			if len(v) == 0 {
				err = ErrInvalidHeaderHost
				return
			}
			h.Host = v
			h.headerFlags |= _hdrFlagHost

		case bytes.Equal(k, []byte(_hdrKeyConnection)):
			if h.headerFlags&_hdrFlagConn == _hdrFlagConn {
				err = ErrInvalidHeaderConn
				return
			}
			if !h.headerValueContains(v, []byte(_hdrValConnectionUpgrade)) {
				err = ErrInvalidHeaderConn
				return
			}
			h.headerFlags |= _hdrFlagConn

		case bytes.Equal(k, []byte(_hdrKeyUpgrade)):
			if h.headerFlags&_hdrFlagUpgrade == _hdrFlagUpgrade {
				err = ErrInvalidHeaderUpgrade
				return
			}
			if !h.headerValueContains(v, []byte(_hdrValUpgradeWS)) {
				err = ErrInvalidHeaderUpgrade
				return
			}
			h.headerFlags |= _hdrFlagUpgrade

		case bytes.Equal(k, []byte(_hdrKeyWSKey)):
			if h.headerFlags&_hdrFlagWSKey == _hdrFlagWSKey {
				err = ErrInvalidHeaderWSKey
				return
			}
			if len(v) == 0 {
				err = ErrInvalidHeaderWSKey
				return
			}
			h.Key = v
			if len(h.Key) != 24 {
				err = ErrInvalidHeaderWSKey
				return
			}
			h.headerFlags |= _hdrFlagWSKey

		case bytes.Equal(k, []byte(_hdrKeyWSVersion)):
			if h.headerFlags&_hdrFlagWSVersion == _hdrFlagWSVersion {
				err = ErrInvalidHeaderWSVersion
				return
			}
			if len(v) == 0 {
				err = ErrInvalidHeaderWSVersion
				return
			}
			if !bytes.Equal(v, []byte(_hdrValWSVersion)) {
				err = ErrUnsupportedWSVersion
				return
			}
			h.headerFlags |= _hdrFlagWSVersion

		case bytes.Equal(k, []byte(_hdrKeyWSExtensions)):
			if h.headerFlags&_hdrFlagWSExtensions == _hdrFlagWSExtensions {
				err = ErrInvalidHeaderWSExtensions
				return
			}
			h.Extensions = v
			h.headerFlags |= _hdrFlagWSExtensions

		case bytes.Equal(k, []byte(_hdrKeyWSProtocol)):
			if h.headerFlags&_hdrFlagWSProtocol == _hdrFlagWSProtocol {
				err = ErrInvalidHeaderWSProtocol
				return
			}
			h.Protocol = v
			h.headerFlags |= _hdrFlagWSProtocol

		default:
			if h.Header == nil {
				h.Header = make(http.Header)
				h.Header.Add(string(k), string(v))
			} else {
				h.Header.Add(string(k), string(v))
			}
		}
	}

	requiredFlags := _hdrFlagHost | _hdrFlagConn | _hdrFlagUpgrade | _hdrFlagWSKey | _hdrFlagWSVersion

	if h.headerFlags&requiredFlags != requiredFlags {
		err = ErrMissingRequiredHeader
		return
	}

	return
}
