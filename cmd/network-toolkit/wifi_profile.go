package main

import (
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

const localWiFiProfileNamespace = "http://www.microsoft.com/networking/WLAN/profile/v1"

// The native profile XML stays private. Only the selected, explicit reveal
// parses keyMaterial; metadata parsing skips it, including encrypted blobs.
type localWiFiParsedProfile struct {
	name, ssid, ssidHex, authentication, encryption, keyType string
	oneX, sharedKey, protected, protectedKnown               bool
	secret                                                   string
}

func localWiFiSSID(value []byte) (string, string) {
	hexValue := strings.ToUpper(hex.EncodeToString(value))
	if len(value) == 0 {
		return "", ""
	}
	readable := utf8.Valid(value)
	if readable {
		for _, c := range string(value) {
			if unicode.IsControl(c) {
				readable = false
				break
			}
		}
	}
	if !readable {
		return "SSID " + hexValue, hexValue
	}
	return string(value), hexValue
}

func localWiFiXMLText(dec *xml.Decoder, start xml.StartElement, limit int) (string, error) {
	var out []byte
	defer func() { clear(out) }()
	for {
		token, err := dec.Token()
		if err != nil {
			return "", errors.New("Windows returned an invalid Wi-Fi profile.")
		}
		switch t := token.(type) {
		case xml.CharData:
			if len(out)+len(t) > limit {
				return "", errors.New("A Wi-Fi profile field exceeds its supported size.")
			}
			out = append(out, t...)
		case xml.Comment:
		case xml.EndElement:
			if t.Name != start.Name {
				return "", errors.New("Windows returned an invalid Wi-Fi profile.")
			}
			return string(out), nil
		default:
			return "", errors.New("Windows returned an invalid Wi-Fi profile field.")
		}
	}
}

func parseLocalWiFiProfile(data []byte, reveal bool) (out localWiFiParsedProfile, resultErr error) {
	defer func() {
		if resultErr != nil {
			out.secret = ""
		}
	}()
	invalid := errors.New("Windows returned an invalid or unsupported Wi-Fi profile.")
	if len(data) == 0 || len(data) > localWiFiXMLLimit {
		return out, invalid
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	// WlanGetProfile returns UTF-16. The native provider has already converted
	// the buffer to UTF-8, while a declaration can still mention UTF-16.
	dec.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		if strings.EqualFold(label, "utf-16") || strings.EqualFold(label, "utf-8") {
			return input, nil
		}
		return nil, invalid
	}
	var path []string
	seen := map[string]bool{}
	root, finished := false, false
	ssidName, ssidHex := "", ""
	for {
		token, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return out, invalid // Never echo parser text containing key material.
		}
		switch t := token.(type) {
		case xml.Directive:
			return out, invalid
		case xml.StartElement:
			if !root {
				if t.Name.Local != "WLANProfile" || t.Name.Space != localWiFiProfileNamespace {
					return out, invalid
				}
				root = true
				path = append(path, "WLANProfile")
				continue
			}
			if finished || len(path) == 0 || len(path) > 24 {
				return out, invalid
			}
			// In particular, never treat an EAP/OneX extension's password or an
			// identically named foreign-namespace element as a shared WLAN key.
			if t.Name.Space != localWiFiProfileNamespace {
				if dec.Skip() != nil {
					return out, invalid
				}
				continue
			}
			path = append(path, t.Name.Local)
			key := strings.Join(path, "/")
			leaf, limit := true, 4096
			switch key {
			case "WLANProfile/name", "WLANProfile/SSIDConfig/SSID/name", "WLANProfile/SSIDConfig/SSID/hex",
				"WLANProfile/MSM/security/authEncryption/authentication", "WLANProfile/MSM/security/authEncryption/encryption",
				"WLANProfile/MSM/security/authEncryption/useOneX", "WLANProfile/MSM/security/sharedKey/keyType",
				"WLANProfile/MSM/security/sharedKey/protected":
			case "WLANProfile/MSM/security/sharedKey/keyMaterial":
				if !reveal {
					if seen[key] || dec.Skip() != nil {
						return out, invalid
					}
					seen[key] = true
					path = path[:len(path)-1]
					continue
				}
				limit = localWiFiXMLLimit // A protected blob can be much larger than a key.
			default:
				leaf = false
			}
			if !leaf {
				switch key {
				case "WLANProfile/SSIDConfig", "WLANProfile/SSIDConfig/SSID", "WLANProfile/MSM", "WLANProfile/MSM/security",
					"WLANProfile/MSM/security/authEncryption", "WLANProfile/MSM/security/sharedKey":
					if seen[key] {
						return out, invalid
					}
					seen[key] = true
				default:
					if dec.Skip() != nil {
						return out, invalid
					}
					path = path[:len(path)-1]
					continue
				}
				if key == "WLANProfile/MSM/security/sharedKey" {
					if out.sharedKey {
						return out, invalid
					}
					out.sharedKey = true
				}
				continue
			}
			if seen[key] {
				return out, invalid
			}
			seen[key] = true
			value, e := localWiFiXMLText(dec, t, limit)
			if e != nil {
				return out, e
			}
			path = path[:len(path)-1]
			switch key {
			case "WLANProfile/name":
				out.name = value
			case "WLANProfile/SSIDConfig/SSID/name":
				ssidName = value
			case "WLANProfile/SSIDConfig/SSID/hex":
				ssidHex = strings.TrimSpace(value)
			case "WLANProfile/MSM/security/authEncryption/authentication":
				out.authentication = strings.TrimSpace(value)
			case "WLANProfile/MSM/security/authEncryption/encryption":
				out.encryption = strings.TrimSpace(value)
			case "WLANProfile/MSM/security/authEncryption/useOneX":
				v := strings.ToLower(strings.TrimSpace(value))
				if v != "true" && v != "false" && v != "1" && v != "0" {
					return out, invalid
				}
				out.oneX = v == "true" || v == "1"
			case "WLANProfile/MSM/security/sharedKey/keyType":
				out.keyType = strings.TrimSpace(value)
			case "WLANProfile/MSM/security/sharedKey/protected":
				v := strings.ToLower(strings.TrimSpace(value))
				if v != "true" && v != "false" && v != "1" && v != "0" {
					return out, invalid
				}
				out.protected, out.protectedKnown = v == "true" || v == "1", true
			case "WLANProfile/MSM/security/sharedKey/keyMaterial":
				out.secret = value // Spaces are part of a passphrase.
			}
		case xml.EndElement:
			if len(path) == 0 || path[len(path)-1] != t.Name.Local || t.Name.Space != localWiFiProfileNamespace {
				return out, invalid
			}
			path = path[:len(path)-1]
			if len(path) == 0 {
				finished = true
			}
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return out, invalid
			}
		}
	}
	if !root || !finished || len(path) != 0 || out.name == "" || out.authentication == "" || out.encryption == "" {
		return out, invalid
	}
	ssid := []byte(ssidName)
	if ssidHex != "" {
		var e error
		ssid, e = hex.DecodeString(ssidHex)
		if e != nil {
			return out, invalid
		}
	}
	if len(ssid) == 0 || len(ssid) > 32 {
		return out, invalid
	}
	out.ssid, out.ssidHex = localWiFiSSID(ssid)
	return out, nil
}

// See the Microsoft WLAN sharedKey schema. A raw WPA PSK and a WEP key are
// not the original human-entered passphrase, and enterprise profiles have no
// shared personal-network password to show.
func localWiFiKeyKind(p localWiFiParsedProfile) (kind, note string) {
	auth, encryption := strings.ToUpper(p.authentication), strings.ToUpper(p.encryption)
	if p.oneX || auth == "WPA" || auth == "WPA2" || auth == "WPA3ENT" || auth == "WPA3ENT192" || auth == "WPA3" {
		return "enterprise", "Enterprise authentication uses an account or certificate; no shared Wi-Fi passphrase is available."
	}
	if encryption == "NONE" || auth == "OWE" || !p.sharedKey {
		return "none", "This profile has no stored shared Wi-Fi key."
	}
	if encryption == "WEP" && p.keyType == "networkKey" {
		return "wep_key", "Windows may return a WEP key as hexadecimal instead of the originally entered characters."
	}
	personal := auth == "WPAPSK" || auth == "WPA2PSK" || auth == "WPA3SAE" || auth == "WPANONE"
	if personal && p.keyType == "passPhrase" {
		return "passphrase", ""
	}
	if (auth == "WPAPSK" || auth == "WPA2PSK" || auth == "WPANONE") && p.keyType == "networkKey" {
		return "raw_psk", "This profile stores a raw pre-shared key; the original passphrase cannot be recovered from it."
	}
	return "unknown", "This profile uses an unsupported shared-key format."
}

func localWiFiScope(flags uint32) string {
	if flags&1 != 0 {
		return "group-policy"
	}
	if flags&2 != 0 {
		return "user"
	}
	return "all-user"
}

func localWiFiFingerprint(id string, ref localWiFiProfileRef, p *localWiFiParsedProfile) string {
	var metadata any
	if p != nil {
		metadata = struct {
			SSID, Authentication, Encryption, KeyType string
			OneX, SharedKey                           bool
		}{p.ssidHex, p.authentication, p.encryption, p.keyType, p.oneX, p.sharedKey}
	}
	return localRevision(struct {
		InterfaceID string
		ProfileName string
		Flags       uint32
		Metadata    any
	}{strings.ToLower(id), ref.Name, ref.Flags, metadata})
}
