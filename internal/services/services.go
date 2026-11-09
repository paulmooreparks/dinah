// Package services is the registry of the page services a build adds
// beside the App's own methods (docs/design/product-applets.md). A product
// built from this source registers a service under a name at start, and
// the page calls its methods through one App method, CallService, by the
// service's name, the method's name and the arguments as a JSON array.
// Nothing in this package knows a service: the product's Go code registers
// what its applets need, and its page files call it.
//
// Object makes a service from any value, offering its exported methods:
// each method's arguments are read from the JSON array in order, its first
// result is the answer, and an error result that is not nil is the
// failure. That is the shape the App's own methods have, so a product's
// service is written as the App's methods are, on a type of its own.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"sync"
)

// Service answers the page's calls of its methods.
type Service interface {
	// Call runs the method with the arguments, a JSON array, and returns
	// the answer, which is sent to the page as JSON.
	Call(ctx context.Context, method string, args json.RawMessage) (any, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Service{}
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Register registers a service under its name, lowercase words joined by
// hyphens. It panics on a name used twice or malformed, since both are
// mistakes in the build.
func Register(name string, s Service) {
	if !namePattern.MatchString(name) {
		panic("services: invalid service name " + name)
	}
	if s == nil {
		panic("services: nil service " + name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, taken := registry[name]; taken {
		panic("services: service registered twice: " + name)
	}
	registry[name] = s
}

// Lookup returns the service registered under a name.
func Lookup(name string) (Service, bool) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := registry[name]
	return s, ok
}

// Names lists the registered services, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ErrUnknownMethod is the failure of a call of a method a service lacks.
var ErrUnknownMethod = errors.New("the service has no such method")

var errorType = reflect.TypeFor[error]()
var contextType = reflect.TypeFor[context.Context]()

// object is a service made of a value's exported methods.
type object struct {
	value   reflect.Value
	methods map[string]reflect.Method
}

// Object makes a service of a value's exported methods. A method may take
// a context.Context first, which receives the call's; its other arguments
// are read from the call's JSON array in order, and a missing argument is
// its type's zero value. A method returns nothing, one value, an error, or
// a value and an error.
func Object(v any) Service {
	value := reflect.ValueOf(v)
	t := value.Type()
	methods := map[string]reflect.Method{}
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		if !m.IsExported() {
			continue
		}
		out := m.Type.NumOut()
		if out > 2 || out == 2 && m.Type.Out(1) != errorType {
			panic(fmt.Sprintf("services: method %s.%s returns %d values; a service method returns a value, an error, both or nothing", t, m.Name, out))
		}
		methods[m.Name] = m
	}
	return object{value: value, methods: methods}
}

func (o object) Call(ctx context.Context, method string, args json.RawMessage) (any, error) {
	m, ok := o.methods[method]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownMethod, method)
	}
	var raw []json.RawMessage
	if len(args) > 0 {
		if err := json.Unmarshal(args, &raw); err != nil {
			return nil, fmt.Errorf("the arguments of %s must be a JSON array: %w", method, err)
		}
	}
	// The receiver is the method's first parameter.
	in := make([]reflect.Value, 0, m.Type.NumIn())
	in = append(in, o.value)
	next := 0
	for i := 1; i < m.Type.NumIn(); i++ {
		p := m.Type.In(i)
		if i == 1 && p == contextType {
			if ctx == nil {
				ctx = context.Background()
			}
			in = append(in, reflect.ValueOf(ctx))
			continue
		}
		arg := reflect.New(p)
		if next < len(raw) && len(raw[next]) > 0 && string(raw[next]) != "null" {
			if err := json.Unmarshal(raw[next], arg.Interface()); err != nil {
				return nil, fmt.Errorf("argument %d of %s: %w", next+1, method, err)
			}
		}
		in = append(in, arg.Elem())
		next++
	}
	if next < len(raw) {
		return nil, fmt.Errorf("%s takes %d arguments; %d were given", method, next, len(raw))
	}
	out := m.Func.Call(in)
	var result any
	var err error
	switch len(out) {
	case 1:
		if m.Type.Out(0) == errorType {
			if !out[0].IsNil() {
				err = out[0].Interface().(error)
			}
		} else {
			result = out[0].Interface()
		}
	case 2:
		result = out[0].Interface()
		if !out[1].IsNil() {
			err = out[1].Interface().(error)
		}
	}
	return result, err
}
