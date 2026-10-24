package deviceinfo

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"unicode"
)

// parseDescription reads only direct fields of the root device. Descriptions of
// embedded devices, services, icons, serial numbers, and control URLs are ignored.
func parseDescription(raw []byte) (Info, error) {
	if len(raw) == 0 || len(raw) > maxDescriptionBytes {
		return Info{}, errors.New("invalid device description size")
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	depth := 0
	rootSeen, rootClosed, deviceSeen, inDevice := false, false, false, false
	namespace, capture := "", ""
	values := map[string]string{}
	var value strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Info{}, errors.New("malformed device description XML")
		}
		switch token := token.(type) {
		case xml.Directive:
			return Info{}, errors.New("device description XML directives are not supported")
		case xml.StartElement:
			depth++
			if depth > 32 {
				return Info{}, errors.New("device description XML nesting exceeded the limit")
			}
			if capture != "" {
				return Info{}, errors.New("device description identity field contains nested XML")
			}
			if depth == 1 {
				if rootSeen || token.Name.Local != "root" || (token.Name.Space != "" && token.Name.Space != "urn:schemas-upnp-org:device-1-0") {
					return Info{}, errors.New("response is not a UPnP root-device description")
				}
				rootSeen, namespace = true, token.Name.Space
			}
			if depth == 2 && token.Name.Local == "device" && token.Name.Space == namespace {
				if deviceSeen {
					return Info{}, errors.New("device description contains multiple root devices")
				}
				deviceSeen, inDevice = true, true
			}
			if depth == 3 && inDevice && token.Name.Space == namespace {
				switch token.Name.Local {
				case "manufacturer", "modelName", "modelNumber", "friendlyName", "deviceType":
					if _, exists := values[token.Name.Local]; exists {
						return Info{}, errors.New("device description repeats an identity field")
					}
					capture = token.Name.Local
					value.Reset()
				}
			}
		case xml.CharData:
			if capture != "" {
				if value.Len()+len(token) > 2048 {
					return Info{}, errors.New("device description identity field exceeded the limit")
				}
				value.Write(token)
			} else if depth == 0 && strings.TrimSpace(string(token)) != "" {
				return Info{}, errors.New("unexpected text outside device description XML")
			}
		case xml.EndElement:
			if depth == 3 && capture != "" {
				values[capture] = cleanIdentityText(value.String())
				capture = ""
			}
			if depth == 2 && inDevice {
				inDevice = false
			}
			if depth == 1 {
				rootClosed = true
			}
			depth--
		}
	}
	deviceType := values["deviceType"]
	if !rootSeen || !rootClosed || !deviceSeen || depth != 0 || !strings.HasPrefix(deviceType, "urn:") || !strings.Contains(deviceType, ":device:") {
		return Info{}, errors.New("response has no valid UPnP root-device description")
	}
	info := Info{Manufacturer: values["manufacturer"], ModelName: values["modelName"], ModelNumber: values["modelNumber"],
		FriendlyName: values["friendlyName"], ModelSource: "UPnP device description"}
	info.Model = info.ModelName
	if info.Model == "" {
		info.Model = info.ModelNumber
	} else if info.ModelNumber != "" && !strings.EqualFold(info.Model, info.ModelNumber) {
		info.Model += " (" + info.ModelNumber + ")"
	}
	return info, nil
}

func cleanIdentityText(raw string) string {
	var out strings.Builder
	for _, r := range strings.Join(strings.Fields(raw), " ") {
		if !unicode.IsControl(r) && !unicode.In(r, unicode.Cf) {
			out.WriteRune(r)
		}
	}
	return out.String()
}
