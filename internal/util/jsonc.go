package util

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf16"
)

// OrderedMap preserves JSON object key order so we never reshuffle user config.
type OrderedMap struct {
	keys   []string
	values map[string]any
}

func NewOrderedMap() *OrderedMap {
	return &OrderedMap{values: map[string]any{}}
}

func (m *OrderedMap) Get(k string) (any, bool) { v, ok := m.values[k]; return v, ok }

func (m *OrderedMap) Set(k string, v any) {
	if _, ok := m.values[k]; !ok {
		m.keys = append(m.keys, k)
	}
	m.values[k] = v
}

func (m *OrderedMap) Delete(k string) {
	if _, ok := m.values[k]; !ok {
		return
	}
	delete(m.values, k)
	for i, kk := range m.keys {
		if kk == k {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			break
		}
	}
}

func (m *OrderedMap) Keys() []string { return m.keys }
func (m *OrderedMap) Len() int       { return len(m.keys) }

// UnmarshalJSON decodes preserving key order via a streaming token reader.
func (m *OrderedMap) UnmarshalJSON(data []byte) error {
	m.values = map[string]any{}
	m.keys = nil
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := t.(json.Delim); !ok || d != '{' {
		return json.Unmarshal(data, &m.values)
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return err
		}
		key := keyTok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		val, err := decodeOrdered(raw)
		if err != nil {
			return err
		}
		m.Set(key, val)
	}
	_, err = dec.Token() // closing }
	return err
}

func decodeOrdered(raw json.RawMessage) (any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		om := NewOrderedMap()
		if err := om.UnmarshalJSON(raw); err != nil {
			return nil, err
		}
		return om, nil
	}
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, err
		}
		out := make([]any, len(arr))
		for i, el := range arr {
			v, err := decodeOrdered(el)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// MarshalJSON writes object keys in preserved order.
func (m *OrderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(m.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// stripComments removes // and /* */ comments and trailing commas.
func stripComments(s string) string {
	s = strings.TrimPrefix(s, "\ufeff")
	out := make([]byte, 0, len(s))
	i, n := 0, len(s)
	inStr, esc := false, false
	var strCh byte
	for i < n {
		ch := s[i]
		if inStr {
			out = append(out, ch)
			if esc {
				esc = false
			} else if ch == '\\' {
				esc = true
			} else if ch == strCh {
				inStr = false
			}
			i++
			continue
		}
		if ch == '"' || ch == '\'' {
			inStr = true
			strCh = ch
			out = append(out, ch)
			i++
			continue
		}
		if ch == '/' && i+1 < n && s[i+1] == '/' {
			for i < n && s[i] != '\n' {
				i++
			}
			continue
		}
		if ch == '/' && i+1 < n && s[i+1] == '*' {
			i += 2
			for i+1 < n && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			i += 2
			continue
		}
		if ch == '}' || ch == ']' {
			last := len(bytes.TrimRight(out, " \t\r\n")) - 1
			if last >= 0 && out[last] == ',' {
				out[last] = ' '
			}
		}
		out = append(out, ch)
		i++
	}
	return string(out)
}

// NormalizeJSONText strips a UTF-8 BOM and decodes UTF-16 text from Windows editors.
func NormalizeJSONText(s string) string {
	raw := []byte(s)
	switch {
	case bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}):
		return s[3:]
	case bytes.HasPrefix(raw, []byte{0xFF, 0xFE}):
		return decodeUTF16(raw[2:], binary.LittleEndian)
	case bytes.HasPrefix(raw, []byte{0xFE, 0xFF}):
		return decodeUTF16(raw[2:], binary.BigEndian)
	case len(raw) >= 4 && raw[1] == 0 && raw[3] == 0:
		return decodeUTF16(raw, binary.LittleEndian)
	case len(raw) >= 4 && raw[0] == 0 && raw[2] == 0:
		return decodeUTF16(raw, binary.BigEndian)
	}
	return s
}

func decodeUTF16(b []byte, order binary.ByteOrder) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u = append(u, order.Uint16(b[i:]))
	}
	return string(utf16.Decode(u))
}

// ParseJsonc parses JSON-with-comments into an ordered map.
func ParseJsonc(text string) (*OrderedMap, error) {
	stripped := stripComments(NormalizeJSONText(text))
	m := NewOrderedMap()
	if strings.TrimSpace(stripped) == "" {
		return m, nil
	}
	if err := json.Unmarshal([]byte(stripped), m); err != nil {
		return nil, err
	}
	return m, nil
}

// TryParseJsonc returns nil on any parse error.
func TryParseJsonc(text string) *OrderedMap {
	m, err := ParseJsonc(text)
	if err != nil {
		return nil
	}
	return m
}

// HasJSONCComments reports comment markers outside JSON strings.
func HasJSONCComments(raw string) bool {
	raw = NormalizeJSONText(raw)
	inString, escaped := false, false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == '/' && i+1 < len(raw) && (raw[i+1] == '/' || raw[i+1] == '*') {
			return true
		}
	}
	return false
}

// StringifyJSON matches JSON.stringify(obj, null, 2) + "\n".
func StringifyJSON(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return buf.String()
}

type jsoncTopProp struct {
	key      string
	keyStart int
	valStart int
	valEnd   int
	commaIdx int
}

func isJSONCWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func skipJSONCTrivia(s string, i int) (int, error) {
	n := len(s)
	for i < n {
		c := s[i]
		if isJSONCWhitespace(c) {
			i++
			continue
		}
		if c == '/' && i+1 < n {
			if s[i+1] == '/' {
				i += 2
				for i < n && s[i] != '\n' {
					i++
				}
				continue
			}
			if s[i+1] == '*' {
				end := strings.Index(s[i+2:], "*/")
				if end < 0 {
					return 0, fmt.Errorf("unterminated block comment")
				}
				i = i + 2 + end + 2
				continue
			}
		}
		break
	}
	return i, nil
}

func scanJSONCString(s string, start int) (string, int, error) {
	n := len(s)
	if start >= n || s[start] != '"' {
		return "", 0, fmt.Errorf("expected string at offset %d", start)
	}
	esc := false
	for i := start + 1; i < n; i++ {
		ch := s[i]
		if esc {
			esc = false
			continue
		}
		if ch == '\\' {
			esc = true
			continue
		}
		if ch == '"' {
			end := i + 1
			var decoded string
			if err := json.Unmarshal([]byte(s[start:end]), &decoded); err != nil {
				return "", 0, err
			}
			return decoded, end, nil
		}
	}
	return "", 0, fmt.Errorf("unterminated string literal")
}

func scanJSONCValueEnd(s string, start int) (int, error) {
	n := len(s)
	if start >= n {
		return 0, fmt.Errorf("unexpected EOF while scanning JSON value")
	}
	switch s[start] {
	case '"':
		_, end, err := scanJSONCString(s, start)
		return end, err
	case '{', '[':
		depth := 0
		for i := start; i < n; {
			switch c := s[i]; c {
			case '"':
				_, next, err := scanJSONCString(s, i)
				if err != nil {
					return 0, err
				}
				i = next
			case '/', ' ', '\t', '\r', '\n':
				if c == '/' && i+1 < n && (s[i+1] == '/' || s[i+1] == '*') {
					next, err := skipJSONCTrivia(s, i)
					if err != nil {
						return 0, err
					}
					i = next
				} else {
					i++
				}
			case '{', '[':
				depth++
				i++
			case '}', ']':
				depth--
				i++
				if depth == 0 {
					return i, nil
				}
			default:
				i++
			}
		}
		return 0, fmt.Errorf("unterminated composite JSON value")
	default:
		i := start
		for i < n {
			c := s[i]
			if isJSONCWhitespace(c) || c == ',' || c == '}' || c == ']' {
				break
			}
			if c == '/' && i+1 < n && (s[i+1] == '/' || s[i+1] == '*') {
				break
			}
			i++
		}
		if i == start {
			return 0, fmt.Errorf("invalid JSON value at offset %d", start)
		}
		return i, nil
	}
}

func scanTopLevelJSONCProps(s string) (openBrace, closeBrace int, props []jsoncTopProp, err error) {
	i, err := skipJSONCTrivia(s, 0)
	if err != nil {
		return 0, 0, nil, err
	}
	if i >= len(s) || s[i] != '{' {
		return 0, 0, nil, fmt.Errorf("expected top-level JSON object")
	}
	openBrace = i
	i++

	for {
		i, err = skipJSONCTrivia(s, i)
		if err != nil {
			return 0, 0, nil, err
		}
		if i >= len(s) {
			return 0, 0, nil, fmt.Errorf("unterminated top-level JSON object")
		}
		if s[i] == '}' {
			closeBrace = i
			break
		}
		keyStart := i
		key, keyEnd, err := scanJSONCString(s, keyStart)
		if err != nil {
			return 0, 0, nil, err
		}
		colonIdx, err := skipJSONCTrivia(s, keyEnd)
		if err != nil {
			return 0, 0, nil, err
		}
		if colonIdx >= len(s) || s[colonIdx] != ':' {
			return 0, 0, nil, fmt.Errorf("expected ':' after key %q", key)
		}
		valStart, err := skipJSONCTrivia(s, colonIdx+1)
		if err != nil {
			return 0, 0, nil, err
		}
		valEnd, err := scanJSONCValueEnd(s, valStart)
		if err != nil {
			return 0, 0, nil, err
		}
		afterVal, err := skipJSONCTrivia(s, valEnd)
		if err != nil {
			return 0, 0, nil, err
		}
		if afterVal >= len(s) {
			return 0, 0, nil, fmt.Errorf("unexpected EOF after value for key %q", key)
		}
		commaIdx := -1
		switch s[afterVal] {
		case ',':
			commaIdx = afterVal
			i = afterVal + 1
		case '}':
			i = afterVal
		default:
			return 0, 0, nil, fmt.Errorf("expected ',' or '}' after key %q", key)
		}
		props = append(props, jsoncTopProp{
			key:      key,
			keyStart: keyStart,
			valStart: valStart,
			valEnd:   valEnd,
			commaIdx: commaIdx,
		})
	}

	tail, err := skipJSONCTrivia(s, closeBrace+1)
	if err != nil {
		return 0, 0, nil, err
	}
	if tail != len(s) {
		return 0, 0, nil, fmt.Errorf("unexpected trailing content at offset %d", tail)
	}
	return openBrace, closeBrace, props, nil
}

func marshalJSONInline(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

func lineIndentAt(s string, pos int) (indent string, lineStart int, onlySpaceBefore bool) {
	idx := strings.LastIndexByte(s[:pos], '\n')
	lineStart = 0
	if idx >= 0 {
		lineStart = idx + 1
	}
	prefix := s[lineStart:pos]
	for i := 0; i < len(prefix); i++ {
		if prefix[i] != ' ' && prefix[i] != '\t' {
			return "", lineStart, false
		}
	}
	return prefix, lineStart, true
}

func detectJSONCStyle(s string, props []jsoncTopProp) (indent, eol string) {
	eol = "\n"
	if strings.Contains(s, "\r\n") {
		eol = "\r\n"
	}
	indent = "  "
	for _, p := range props {
		if ind, _, ok := lineIndentAt(s, p.keyStart); ok && ind != "" {
			return ind, eol
		}
	}
	return indent, eol
}

// SetJSONCTopLevelValue updates or inserts a top-level key in JSON/JSONC text while
// preserving comments, key order, and formatting.
func SetJSONCTopLevelValue(raw, key string, value any) (string, error) {
	s := NormalizeJSONText(raw)
	valJSON, err := marshalJSONInline(value)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(s) == "" {
		keyJSON, err := marshalJSONInline(key)
		if err != nil {
			return "", err
		}
		out := "{\n  " + keyJSON + ": " + valJSON + "\n}\n"
		if _, perr := ParseJsonc(out); perr != nil {
			return "", perr
		}
		return out, nil
	}

	openBrace, closeBrace, props, err := scanTopLevelJSONCProps(s)
	if err != nil {
		return "", err
	}
	for _, p := range props {
		if p.key == key {
			out := s[:p.valStart] + valJSON + s[p.valEnd:]
			if _, perr := ParseJsonc(out); perr != nil {
				return "", perr
			}
			return out, nil
		}
	}

	keyJSON, err := marshalJSONInline(key)
	if err != nil {
		return "", err
	}
	indent, eol := detectJSONCStyle(s, props)

	var out string
	if len(props) == 0 {
		if !strings.Contains(s[openBrace:closeBrace], "\n") {
			out = s[:openBrace+1] + eol + indent + keyJSON + ": " + valJSON + eol + s[closeBrace:]
		} else {
			_, closeLineStart, _ := lineIndentAt(s, closeBrace)
			if closeLineStart <= openBrace {
				closeLineStart = closeBrace
			}
			out = s[:closeLineStart] + indent + keyJSON + ": " + valJSON + eol + s[closeLineStart:]
		}
	} else {
		last := props[len(props)-1]
		if !strings.Contains(s[openBrace:closeBrace], "\n") {
			prefix := s[:last.valEnd]
			suffix := s[last.valEnd:]
			if last.commaIdx >= 0 {
				prefix = s[:last.commaIdx+1]
				suffix = s[last.commaIdx+1:]
			} else {
				prefix += ","
			}
			out = prefix + " " + keyJSON + ": " + valJSON + suffix
		} else {
			work := s
			if last.commaIdx < 0 {
				work = work[:last.valEnd] + "," + work[last.valEnd:]
				closeBrace++
			}
			_, closeLineStart, _ := lineIndentAt(work, closeBrace)
			if closeLineStart <= openBrace {
				closeLineStart = closeBrace
			}
			out = work[:closeLineStart] + indent + keyJSON + ": " + valJSON + eol + work[closeLineStart:]
		}
	}

	if _, perr := ParseJsonc(out); perr != nil {
		return "", perr
	}
	return out, nil
}

// lineTailIfOwnProperty returns the byte offset immediately after the EOL following endPos
// if only horizontal whitespace or a // comment remains before EOL.
func lineTailIfOwnProperty(s string, endPos int) (int, bool) {
	i := endPos
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i < len(s)-1 && s[i] == '/' && s[i+1] == '/' {
		for i < len(s) && s[i] != '\n' {
			i++
		}
	}
	if i < len(s) && s[i] == '\r' {
		i++
	}
	if i < len(s) && s[i] == '\n' {
		return i + 1, true
	}
	return endPos, false
}

// DeleteJSONCTopLevelKey removes a top-level key from JSON/JSONC text while preserving
// unrelated comments and formatting.
func DeleteJSONCTopLevelKey(raw, key string) (string, bool, error) {
	s := NormalizeJSONText(raw)
	if strings.TrimSpace(s) == "" {
		return raw, false, nil
	}
	_, _, props, err := scanTopLevelJSONCProps(s)
	if err != nil {
		return "", false, err
	}
	targetIdx := -1
	for i, p := range props {
		if p.key == key {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return raw, false, nil
	}

	p := props[targetIdx]
	endPos := p.valEnd
	if p.commaIdx >= 0 {
		endPos = p.commaIdx + 1
	}

	_, lineStart, soleOnLeft := lineIndentAt(s, p.keyStart)
	tailPos, soleOnRight := lineTailIfOwnProperty(s, endPos)

	delStart, delEnd := p.keyStart, endPos
	if soleOnLeft && soleOnRight {
		delStart = lineStart
		delEnd = tailPos
	} else if !soleOnLeft && delStart > 0 && s[delStart-1] == ' ' {
		for delStart > 0 && s[delStart-1] == ' ' {
			delStart--
		}
	}

	out := s[:delStart] + s[delEnd:]
	if p.commaIdx < 0 && targetIdx > 0 {
		prevComma := props[targetIdx-1].commaIdx
		if prevComma >= 0 && prevComma < delStart {
			out = out[:prevComma] + out[prevComma+1:]
		}
	}

	if _, perr := ParseJsonc(out); perr != nil {
		return "", false, perr
	}
	return out, true, nil
}
