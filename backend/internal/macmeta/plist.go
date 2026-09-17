package macmeta

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

func ParsePlist(data []byte) (map[string]any, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, errors.New("empty plist")
	}
	if bytes.HasPrefix(data, []byte("bplist00")) {
		value, err := parseBinaryPlist(data)
		if err != nil {
			return nil, err
		}
		result, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("plist root is not a dictionary")
		}
		return result, nil
	}
	return parseXMLPlist(data)
}

func parseXMLPlist(data []byte) (map[string]any, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("plist dictionary not found")
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "dict" {
			continue
		}
		value, err := parseXMLDict(decoder)
		if err != nil {
			return nil, err
		}
		return value, nil
	}
}

func parseXMLDict(decoder *xml.Decoder) (map[string]any, error) {
	result := make(map[string]any)
	var key string
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch typed := token.(type) {
		case xml.EndElement:
			if typed.Name.Local == "dict" {
				return result, nil
			}
		case xml.StartElement:
			if typed.Name.Local == "key" {
				var value string
				if err := decoder.DecodeElement(&value, &typed); err != nil {
					return nil, err
				}
				key = value
				continue
			}
			if key == "" {
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
				continue
			}
			value, err := parseXMLValue(decoder, typed)
			if err != nil {
				return nil, err
			}
			result[key] = value
			key = ""
		}
	}
}

func parseXMLValue(decoder *xml.Decoder, start xml.StartElement) (any, error) {
	switch start.Name.Local {
	case "string", "date", "data":
		var value string
		if err := decoder.DecodeElement(&value, &start); err != nil {
			return nil, err
		}
		return strings.TrimSpace(value), nil
	case "integer":
		var raw string
		if err := decoder.DecodeElement(&raw, &start); err != nil {
			return nil, err
		}
		value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return strings.TrimSpace(raw), nil
		}
		return value, nil
	case "real":
		var raw string
		if err := decoder.DecodeElement(&raw, &start); err != nil {
			return nil, err
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return strings.TrimSpace(raw), nil
		}
		return value, nil
	case "true":
		if err := decoder.Skip(); err != nil {
			return nil, err
		}
		return true, nil
	case "false":
		if err := decoder.Skip(); err != nil {
			return nil, err
		}
		return false, nil
	case "dict":
		return parseXMLDict(decoder)
	case "array":
		var values []any
		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			switch typed := token.(type) {
			case xml.EndElement:
				if typed.Name.Local == "array" {
					return values, nil
				}
			case xml.StartElement:
				value, err := parseXMLValue(decoder, typed)
				if err != nil {
					return nil, err
				}
				values = append(values, value)
			}
		}
	default:
		if err := decoder.Skip(); err != nil {
			return nil, err
		}
		return nil, nil
	}
}

type binaryPlistParser struct {
	data       []byte
	offsets    []uint64
	refSize    int
	objectMemo map[uint64]any
	parsing    map[uint64]bool
}

func parseBinaryPlist(data []byte) (any, error) {
	if len(data) < 40 || !bytes.HasPrefix(data, []byte("bplist00")) {
		return nil, errors.New("invalid binary plist")
	}
	trailer := data[len(data)-32:]
	offsetSize := int(trailer[6])
	refSize := int(trailer[7])
	if offsetSize < 1 || offsetSize > 8 || refSize < 1 || refSize > 8 {
		return nil, errors.New("invalid binary plist sizes")
	}
	numObjects := binary.BigEndian.Uint64(trailer[8:16])
	topObject := binary.BigEndian.Uint64(trailer[16:24])
	offsetTableOffset := binary.BigEndian.Uint64(trailer[24:32])
	if numObjects == 0 || topObject >= numObjects || offsetTableOffset >= uint64(len(data)) {
		return nil, errors.New("invalid binary plist trailer")
	}
	if numObjects > 1<<20 {
		return nil, errors.New("binary plist has too many objects")
	}
	tableBytes := numObjects * uint64(offsetSize)
	if offsetTableOffset+tableBytes > uint64(len(data)-32) {
		return nil, errors.New("binary plist offset table is out of bounds")
	}

	offsets := make([]uint64, numObjects)
	for i := uint64(0); i < numObjects; i++ {
		start := offsetTableOffset + i*uint64(offsetSize)
		offsets[i] = readUnsigned(data[start : start+uint64(offsetSize)])
		if offsets[i] >= uint64(len(data)-32) {
			return nil, errors.New("binary plist object offset is out of bounds")
		}
	}

	parser := &binaryPlistParser{
		data:       data,
		offsets:    offsets,
		refSize:    refSize,
		objectMemo: make(map[uint64]any),
		parsing:    make(map[uint64]bool),
	}
	return parser.object(topObject, 0)
}

func (p *binaryPlistParser) object(index uint64, depth int) (any, error) {
	if depth > 64 {
		return nil, errors.New("binary plist nesting is too deep")
	}
	if value, ok := p.objectMemo[index]; ok {
		return value, nil
	}
	if p.parsing[index] {
		return nil, errors.New("binary plist contains a cycle")
	}
	if index >= uint64(len(p.offsets)) {
		return nil, errors.New("binary plist object reference is out of bounds")
	}
	p.parsing[index] = true
	defer delete(p.parsing, index)

	offset := p.offsets[index]
	if offset >= uint64(len(p.data)) {
		return nil, errors.New("binary plist object is out of bounds")
	}
	marker := p.data[offset]
	kind := marker >> 4
	info := marker & 0x0f
	pos := offset + 1

	var value any
	var err error
	switch kind {
	case 0x0:
		switch info {
		case 0x0:
			value = nil
		case 0x8:
			value = false
		case 0x9:
			value = true
		default:
			value = nil
		}
	case 0x1:
		size := uint64(1) << info
		if size > 8 || pos+size > uint64(len(p.data)) {
			return nil, errors.New("invalid binary plist integer")
		}
		value = int64(readUnsigned(p.data[pos : pos+size]))
	case 0x2:
		size := uint64(1) << info
		if pos+size > uint64(len(p.data)) {
			return nil, errors.New("invalid binary plist real")
		}
		switch size {
		case 4:
			value = float64(math.Float32frombits(binary.BigEndian.Uint32(p.data[pos : pos+4])))
		case 8:
			value = math.Float64frombits(binary.BigEndian.Uint64(p.data[pos : pos+8]))
		default:
			return nil, errors.New("unsupported binary plist real")
		}
	case 0x4:
		length, next, lengthErr := p.objectLength(info, pos)
		if lengthErr != nil {
			return nil, lengthErr
		}
		if next+length > uint64(len(p.data)) {
			return nil, errors.New("invalid binary plist data")
		}
		value = append([]byte(nil), p.data[next:next+length]...)
	case 0x5:
		length, next, lengthErr := p.objectLength(info, pos)
		if lengthErr != nil {
			return nil, lengthErr
		}
		if next+length > uint64(len(p.data)) {
			return nil, errors.New("invalid binary plist string")
		}
		value = string(p.data[next : next+length])
	case 0x6:
		length, next, lengthErr := p.objectLength(info, pos)
		if lengthErr != nil {
			return nil, lengthErr
		}
		byteLength := length * 2
		if next+byteLength > uint64(len(p.data)) {
			return nil, errors.New("invalid binary plist unicode string")
		}
		units := make([]uint16, length)
		for i := uint64(0); i < length; i++ {
			units[i] = binary.BigEndian.Uint16(p.data[next+i*2 : next+i*2+2])
		}
		value = string(utf16.Decode(units))
	case 0xa:
		length, next, lengthErr := p.objectLength(info, pos)
		if lengthErr != nil {
			return nil, lengthErr
		}
		if length > 1<<20 || next+length*uint64(p.refSize) > uint64(len(p.data)) {
			return nil, errors.New("invalid binary plist array")
		}
		items := make([]any, 0, length)
		for i := uint64(0); i < length; i++ {
			ref := readUnsigned(p.data[next+i*uint64(p.refSize) : next+(i+1)*uint64(p.refSize)])
			item, itemErr := p.object(ref, depth+1)
			if itemErr != nil {
				return nil, itemErr
			}
			items = append(items, item)
		}
		value = items
	case 0xd:
		length, next, lengthErr := p.objectLength(info, pos)
		if lengthErr != nil {
			return nil, lengthErr
		}
		refsBytes := length * uint64(p.refSize)
		if length > 1<<20 || next+refsBytes*2 > uint64(len(p.data)) {
			return nil, errors.New("invalid binary plist dictionary")
		}
		result := make(map[string]any)
		valuesStart := next + refsBytes
		for i := uint64(0); i < length; i++ {
			keyRef := readUnsigned(p.data[next+i*uint64(p.refSize) : next+(i+1)*uint64(p.refSize)])
			valueRef := readUnsigned(p.data[valuesStart+i*uint64(p.refSize) : valuesStart+(i+1)*uint64(p.refSize)])
			keyValue, keyErr := p.object(keyRef, depth+1)
			if keyErr != nil {
				return nil, keyErr
			}
			key, ok := keyValue.(string)
			if !ok {
				return nil, errors.New("binary plist dictionary key is not a string")
			}
			item, itemErr := p.object(valueRef, depth+1)
			if itemErr != nil {
				return nil, itemErr
			}
			result[key] = item
		}
		value = result
	default:
		err = fmt.Errorf("unsupported binary plist object kind 0x%x", kind)
	}
	if err != nil {
		return nil, err
	}
	p.objectMemo[index] = value
	return value, nil
}

func (p *binaryPlistParser) objectLength(info byte, pos uint64) (uint64, uint64, error) {
	if info < 0x0f {
		return uint64(info), pos, nil
	}
	if pos >= uint64(len(p.data)) {
		return 0, 0, errors.New("binary plist length is out of bounds")
	}
	marker := p.data[pos]
	if marker>>4 != 0x1 {
		return 0, 0, errors.New("binary plist extended length is not an integer")
	}
	size := uint64(1) << (marker & 0x0f)
	if size > 8 || pos+1+size > uint64(len(p.data)) {
		return 0, 0, errors.New("invalid binary plist extended length")
	}
	length := readUnsigned(p.data[pos+1 : pos+1+size])
	return length, pos + 1 + size, nil
}

func readUnsigned(data []byte) uint64 {
	var value uint64
	for _, b := range data {
		value = value<<8 | uint64(b)
	}
	return value
}

func plistString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case int64:
			return strconv.FormatInt(typed, 10)
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64)
		}
	}
	return ""
}
