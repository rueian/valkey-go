package rueidis

import (
	"fmt"
	"strconv"
	"strings"
	"unsafe"
)

const messageStructSize = int(unsafe.Sizeof(RedisMessage{}))

// IsRedisNil is a handy method to check if error is redis nil response.
// All redis nil response returns as an error.
func IsRedisNil(err error) bool {
	if e, ok := err.(*RedisError); ok {
		return e.IsNil()
	}
	return false
}

// RedisError is an error response or a nil message from redis instance
type RedisError RedisMessage

func (r *RedisError) Error() string {
	if r.IsNil() {
		return "redis nil message"
	}
	return r.string
}

func (r *RedisError) IsNil() bool {
	return r.typ == '_'
}

func (r *RedisError) IsMoved() (addr string, ok bool) {
	if ok = strings.HasPrefix(r.string, "MOVED"); ok {
		addr = strings.Split(r.string, " ")[2]
	}
	return
}

func (r *RedisError) IsAsk() (addr string, ok bool) {
	if ok = strings.HasPrefix(r.string, "ASK"); ok {
		addr = strings.Split(r.string, " ")[2]
	}
	return
}

func (r *RedisError) IsTryAgain() bool {
	return strings.HasPrefix(r.string, "TRYAGAIN")
}

func (r *RedisError) IsNoScript() bool {
	return strings.HasPrefix(r.string, "NOSCRIPT")
}

func newResult(val RedisMessage, err error) RedisResult {
	return RedisResult{val: val, err: err}
}

func newErrResult(err error) RedisResult {
	return RedisResult{err: err}
}

// RedisResult is the return struct from Client.Do or Client.DoCache
// it contains either a redis response or an underlying error (ex. network timeout).
type RedisResult struct {
	val RedisMessage
	err error
}

// RedisError can be used to check if the redis response is an error message.
func (r RedisResult) RedisError() *RedisError {
	if err := r.val.Error(); err != nil {
		return err.(*RedisError)
	}
	return nil
}

// NonRedisError can be used to check if there is an underlying error (ex. network timeout).
func (r RedisResult) NonRedisError() error {
	return r.err
}

// Error returns either underlying error or redis error or nil
func (r RedisResult) Error() error {
	if r.err != nil {
		return r.err
	}
	if err := r.val.Error(); err != nil {
		return err
	}
	return nil
}

// ToMessage retrieves the RedisMessage
func (r RedisResult) ToMessage() (RedisMessage, error) {
	return r.val, r.Error()
}

// ToInt64 delegates to RedisMessage.ToInt64
func (r RedisResult) ToInt64() (int64, error) {
	if err := r.Error(); err != nil {
		return 0, err
	}
	return r.val.ToInt64()
}

// ToBool delegates to RedisMessage.ToBool
func (r RedisResult) ToBool() (bool, error) {
	if err := r.Error(); err != nil {
		return false, err
	}
	return r.val.ToBool()
}

// ToFloat64 delegates to RedisMessage.ToFloat64
func (r RedisResult) ToFloat64() (float64, error) {
	if err := r.Error(); err != nil {
		return 0, err
	}
	return r.val.ToFloat64()
}

// ToString delegates to RedisMessage.ToString
func (r RedisResult) ToString() (string, error) {
	if err := r.Error(); err != nil {
		return "", err
	}
	return r.val.ToString()
}

// ToArray delegates to RedisMessage.ToArray
func (r RedisResult) ToArray() ([]RedisMessage, error) {
	if err := r.Error(); err != nil {
		return nil, err
	}
	return r.val.ToArray()
}

// ToMap delegates to RedisMessage.ToMap
func (r RedisResult) ToMap() (map[string]RedisMessage, error) {
	if err := r.Error(); err != nil {
		return nil, err
	}
	return r.val.ToMap()
}

// RedisMessage is a redis response message, it may be a nil response
type RedisMessage struct {
	string  string
	integer int64
	values  []RedisMessage
	attrs   *RedisMessage
	typ     byte
}

// IsNil check if message is a redis nil response
func (m *RedisMessage) IsNil() bool {
	return m.typ == '_'
}

// Error check if message is a redis error response, including nil response
func (m *RedisMessage) Error() error {
	if m.typ == '-' || m.typ == '_' || m.typ == '!' {
		return (*RedisError)(m)
	}
	return nil
}

// ToString check if message is a redis string response, and return it
func (m *RedisMessage) ToString() (val string, err error) {
	if m.typ == '$' || m.typ == '+' {
		return m.string, nil
	}
	if m.typ == ':' || m.values != nil {
		panic(fmt.Sprintf("redis message type %c is not a string", m.typ))
	}
	return m.string, m.Error()
}

// ToInt64 check if message is a redis int response, and return it
func (m *RedisMessage) ToInt64() (val int64, err error) {
	if m.typ == ':' {
		return m.integer, nil
	}
	if err = m.Error(); err != nil {
		return 0, err
	}
	panic(fmt.Sprintf("redis message type %c is not a int64", m.typ))
}

// ToBool check if message is a redis bool response, and return it
func (m *RedisMessage) ToBool() (val bool, err error) {
	if m.typ == '#' {
		return m.integer == 1, nil
	}
	if err = m.Error(); err != nil {
		return false, err
	}
	panic(fmt.Sprintf("redis message type %c is not a bool", m.typ))
}

// ToFloat64 check if message is a redis double response, and return it
func (m *RedisMessage) ToFloat64() (val float64, err error) {
	if m.typ == ',' {
		return strconv.ParseFloat(m.string, 64)
	}
	if err = m.Error(); err != nil {
		return 0, err
	}
	panic(fmt.Sprintf("redis message type %c is not a float64", m.typ))
}

// ToArray check if message is a redis array/set response, and return it
func (m *RedisMessage) ToArray() ([]RedisMessage, error) {
	if m.typ == '*' || m.typ == '~' {
		return m.values, nil
	}
	if err := m.Error(); err != nil {
		return nil, err
	}
	panic(fmt.Sprintf("redis message type %c is not a array", m.typ))
}

// ToMap check if message is a redis map response, and return it
func (m *RedisMessage) ToMap() (map[string]RedisMessage, error) {
	if m.typ == '%' {
		r := make(map[string]RedisMessage, len(m.values)/2)
		for i := 0; i < len(m.values); i += 2 {
			if m.values[i].typ == '$' || m.values[i].typ == '+' {
				r[m.values[i].string] = m.values[i+1]
				continue
			}
			panic(fmt.Sprintf("redis message type %c as map key is not supported by ToMap", m.values[i].typ))
		}
		return r, nil
	}
	if err := m.Error(); err != nil {
		return nil, err
	}
	panic(fmt.Sprintf("redis message type %c is not a map", m.typ))
}

func (m *RedisMessage) approximateSize() (s int) {
	s += messageStructSize
	s += len(m.string)
	for _, v := range m.values {
		s += v.approximateSize()
	}
	if m.attrs != nil {
		s += m.attrs.approximateSize()
	}
	return
}