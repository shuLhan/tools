package net

import (
	libbytes "github.com/shuLhan/share/lib/bytes"
)

//
// IsHostnameValid will return true if hostname is valid, otherwise it will
// return false.
// They must begin with alphanumeric character or "_" and end with an
// alphanumeric character.
// Host names may contain only alphanumeric characters, minus signs ("-"),
// underscore ("_"), and periods (".").
//
// If isFQDN is true, the hostname must at least contains two labels;
// otherwise it will be invalid.
//
// See rfc952 and rfc1123.
//
func IsHostnameValid(hname []byte, isFQDN bool) bool {
	n := len(hname)
	if n == 0 {
		return false
	}
	if !libbytes.IsAlnum(hname[0]) && hname[0] != '_' {
		return false
	}
	if !libbytes.IsAlnum(hname[n-1]) {
		return false
	}
	var ndot int
	for x := 1; x < n-1; x++ {
		if hname[x] == '.' {
			ndot++
			continue
		}
		if hname[x] == '-' || hname[x] == '_' || libbytes.IsAlnum(hname[x]) {
			continue
		}
		return false
	}
	if isFQDN && ndot == 0 {
		return false
	}
	return true
}
