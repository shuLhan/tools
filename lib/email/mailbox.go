// Copyright 2019, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package email

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	libio "github.com/shuLhan/share/lib/io"
	libjson "github.com/shuLhan/share/lib/json"
	libnet "github.com/shuLhan/share/lib/net"
)

const (
	stateBegin       = 1 << iota // 1
	stateDisplayName             // 2
	stateLocalPart               // 4
	stateDomain                  // 8
	stateEnd                     // 16
	stateGroupEnd                // 32
)

//
// Mailbox represent an invidual mailbox.
//
type Mailbox struct {
	Name    []byte
	Local   []byte
	Domain  []byte
	Address string // address contains the combination of "local@domain"
	isAngle bool
}

//
// String return the text representation of mailbox.
//
func (mbox *Mailbox) String() string {
	var sb strings.Builder

	if len(mbox.Name) > 0 {
		sb.Write(mbox.Name)
		sb.WriteByte(' ')
	}
	sb.WriteByte('<')
	sb.Write(mbox.Local)
	if len(mbox.Domain) > 0 {
		sb.WriteByte('@')
		sb.Write(mbox.Domain)
	}
	sb.WriteByte('>')

	return sb.String()
}

//
// ParseMailbox parse the raw address and return a single mailbox, the first
// mailbox in the list.
//
func ParseMailbox(raw []byte) (mbox *Mailbox, err error) {
	mboxes, err := ParseMailboxes(raw)
	if err != nil {
		return nil, err
	}
	if len(mboxes) > 0 {
		return mboxes[0], nil
	}
	return nil, errors.New("empty mailbox")
}

//
// ParseMailboxes parse raw address into single or multiple mailboxes.
// Raw address can be a group of address, list of mailbox, or single mailbox.
//
// A group of address have the following syntax,
//
//	DisplayName ":" mailbox-list ";" [comment]
//
// List of mailbox (mailbox-list) have following syntax,
//
//	mailbox *("," mailbox)
//
// A single mailbox have following syntax,
//
//	[DisplayName] ["<"] local "@" domain [">"]
//
// The angle bracket is optional, but both must be provided.
//
// DisplayName, local, and domain can have comment before and/or after it,
//
//	[comment] text [comment]
//
// A comment have the following syntax,
//
//	"(" text [comment] ")"
//
func ParseMailboxes(raw []byte) (mboxes []*Mailbox, err error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("ParseMailboxes %q: empty address", raw)
	}

	r := &libio.Reader{}
	r.Init(raw)

	var (
		seps    = []byte{'(', ':', '<', '@', '>', ',', ';'}
		tok     []byte
		value   []byte
		isGroup bool
		c       byte
		mbox    *Mailbox
		state   = stateBegin
	)

	_ = r.SkipSpaces()
	tok, _, c = r.ReadUntil(seps, nil)
	for {
		switch c {
		case '(':
			_, err = skipComment(r)
			if err != nil {
				return nil, fmt.Errorf("ParseMailboxes %q: %w", raw, err)
			}
			if len(tok) > 0 {
				value = append(value, tok...)
			}

		case ':':
			if state != stateBegin {
				return nil, fmt.Errorf("ParseMailboxes %q: invalid character: ':'", raw)
			}
			isGroup = true
			value = nil
			state = stateDisplayName
			_ = r.SkipSpaces()

		case '<':
			if state >= stateLocalPart {
				return nil, fmt.Errorf("ParseMailboxes %q: invalid character: '<'", raw)
			}
			value = append(value, tok...)
			value = bytes.TrimSpace(value)
			mbox = &Mailbox{
				isAngle: true,
			}
			if len(value) > 0 {
				mbox.Name = value
			}
			value = nil
			state = stateLocalPart

		case '@':
			if state >= stateDomain {
				return nil, fmt.Errorf("ParseMailboxes %q: invalid character: '@'", raw)
			}
			value = append(value, tok...)
			value = bytes.TrimSpace(value)
			if len(value) == 0 {
				return nil, fmt.Errorf("ParseMailboxes %q: empty local", raw)
			}
			if mbox == nil {
				mbox = &Mailbox{}
			}
			if !IsValidLocal(value) {
				return nil, fmt.Errorf("ParseMailboxes %q: invalid local: '%s'", raw, value)
			}
			mbox.Local = value
			value = nil
			state = stateDomain

		case '>':
			if state > stateDomain || !mbox.isAngle {
				return nil, fmt.Errorf("ParseMailboxes %q: invalid character: '>'", raw)
			}
			value = append(value, tok...)
			value = bytes.TrimSpace(value)
			if state == stateDomain {
				if !libnet.IsHostnameValid(value, false) {
					return nil, fmt.Errorf("ParseMailboxes %q: invalid domain: '%s'", raw, value)
				}
			}
			mbox.Domain = value
			mbox.Address = fmt.Sprintf("%s@%s", mbox.Local, mbox.Domain)
			mboxes = append(mboxes, mbox)
			mbox = nil
			value = nil
			state = stateEnd

		case ';':
			if state < stateDomain || !isGroup {
				return nil, fmt.Errorf("ParseMailboxes %q: invalid character: ';'", raw)
			}
			if mbox != nil && mbox.isAngle {
				return nil, fmt.Errorf("ParseMailboxes %q: missing '>'", raw)
			}
			value = append(value, tok...)
			value = bytes.TrimSpace(value)
			switch state {
			case stateDomain:
				if !libnet.IsHostnameValid(value, false) {
					return nil, fmt.Errorf("ParseMailboxes %q: invalid domain: '%s'", raw, value)
				}
				mbox.Domain = value
				mbox.Address = fmt.Sprintf("%s@%s", mbox.Local, mbox.Domain)
				mboxes = append(mboxes, mbox)
				mbox = nil
			case stateEnd:
				if len(value) > 0 {
					return nil, fmt.Errorf("ParseMailboxes %q: invalid token: '%s'", raw, value)
				}
			}
			isGroup = false
			value = nil
			state = stateGroupEnd
		case ',':
			if state < stateDomain {
				return nil, fmt.Errorf("ParseMailboxes %q: invalid character: ','", raw)
			}
			if mbox != nil && mbox.isAngle {
				return nil, fmt.Errorf("ParseMailboxes %q: missing '>'", raw)
			}
			value = append(value, tok...)
			value = bytes.TrimSpace(value)
			switch state {
			case stateDomain:
				if !libnet.IsHostnameValid(value, false) {
					return nil, fmt.Errorf("ParseMailboxes %q: invalid domain: '%s'", raw, value)
				}
				mbox.Domain = value
				mbox.Address = fmt.Sprintf("%s@%s", mbox.Local, mbox.Domain)
				mboxes = append(mboxes, mbox)
				mbox = nil
			case stateEnd:
				if len(value) > 0 {
					return nil, fmt.Errorf("ParseMailboxes %q: invalid token: '%s'", raw, value)
				}
			}
			value = nil
			state = stateBegin
		case 0:
			if state < stateDomain {
				return nil, fmt.Errorf("ParseMailboxes %q: empty or invalid address", raw)
			}
			if state != stateEnd && mbox != nil && mbox.isAngle {
				return nil, fmt.Errorf("ParseMailboxes %q: missing '>'", raw)
			}
			if isGroup {
				return nil, fmt.Errorf("ParseMailboxes %q: missing ';'", raw)
			}

			value = append(value, tok...)
			value = bytes.TrimSpace(value)
			if state == stateGroupEnd {
				if len(value) > 0 {
					return nil, fmt.Errorf("ParseMailboxes %q: trailing text: '%s'", raw, value)
				}
			}

			if state == stateDomain {
				if !libnet.IsHostnameValid(value, false) {
					return nil, fmt.Errorf("ParseMailboxes %q: invalid domain: '%s'", raw, value)
				}
				mbox.Domain = value
				mbox.Address = fmt.Sprintf("%s@%s", mbox.Local, mbox.Domain)
				mboxes = append(mboxes, mbox)
				mbox = nil
			}
			goto out
		}
		tok, _, c = r.ReadUntil(seps, nil)
	}
out:
	return mboxes, nil
}

//
// skipComment skip all characters inside parentheses, '(' and ')'.
//
// A comment can contains quoted-pair, which means opening or closing
// parentheses can be escaped using backslash character '\', for example
// "( a \) comment)".
//
// A comment can be nested, for example "(a (comment))"
//
func skipComment(r *libio.Reader) (c byte, err error) {
	seps := []byte{'\\', '(', ')'}
	c = r.SkipUntil(seps)
	for {
		switch c {
		case 0:
			return c, errors.New("missing comment close parentheses")
		case '\\':
			// We found backslash, skip one character and continue
			// looking for separator.
			r.SkipN(1)
		case '(':
			c, err = skipComment(r)
			if err != nil {
				return c, err
			}
		case ')':
			c = r.SkipSpaces()
			if c != '(' {
				goto out
			}
			r.SkipN(1)
		}
		c = r.SkipUntil(seps)
	}
out:
	return c, nil
}

func (mbox *Mailbox) UnmarshalJSON(b []byte) (err error) {
	// Replace \u003c and \u003e escaped characters back to '<' and '>'.
	b, err = libjson.Unescape(b, false)
	if err != nil {
		return err
	}
	if b[0] == '"' {
		b = b[1:]
	}
	if b[len(b)-1] == '"' {
		b = b[:len(b)-1]
	}
	got, err := ParseMailbox(b)
	if err != nil {
		return err
	}
	*mbox = *got
	return nil
}

func (mbox *Mailbox) MarshalJSON() (b []byte, err error) {
	return []byte(`"` + mbox.String() + `"`), nil
}
