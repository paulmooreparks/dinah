package screen

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// errUnsupported is a parameterised string this evaluator cannot evaluate,
// which the layer treats as a capability the terminal lacks.
var errUnsupported = errors.New("screen: terminfo string uses an operation this evaluator does not support")

// value is one entry of the evaluator's stack. terminfo(5) pushes numbers
// and, for %s and %l, strings; a parameter here is always a number.
type value struct {
	number   int
	text     string
	isString bool
}

// Evaluate expands a parameterised string with the parameter language
// terminfo(5) documents under "Parameterized Strings": %%, the printf-style
// %[[:]flags][width[.precision]][doxXs], %c, %s, %p1 to %p9, %P and %g for
// the dynamic variables a to z and the static variables A to Z, %'c', %{nn},
// %l, the arithmetic %+ %- %* %/ %m, the bit operations %& %| %^, the
// comparisons %= %> %<, the logical %A %O, the unary %! %~, %i, and the
// conditional %? expr %t then %e else %;. Anything else, and a string that
// pops more than it pushed, is errUnsupported. A division by zero pushes
// zero rather than failing.
//
// The static variables live for one call, since nothing this package draws
// ever reads one that another call set.
func Evaluate(capability string, params ...int) (string, error) {
	var p [9]int
	copy(p[:], params)
	var stack []value
	var dynamic, static [26]int
	var out strings.Builder
	pop := func() (value, error) {
		if len(stack) == 0 {
			return value{}, errUnsupported
		}
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return top, nil
	}
	popNumber := func() (int, error) {
		top, err := pop()
		if err != nil || top.isString {
			return 0, errUnsupported
		}
		return top.number, nil
	}
	push := func(n int) { stack = append(stack, value{number: n}) }
	s := capability
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			out.WriteByte(s[i])
			continue
		}
		i++
		if i >= len(s) {
			return "", errUnsupported
		}
		switch c := s[i]; c {
		case '%':
			out.WriteByte('%')
		case 'c':
			n, err := popNumber()
			if err != nil {
				return "", err
			}
			out.WriteByte(byte(n))
		case 'p':
			i++
			if i >= len(s) || s[i] < '1' || s[i] > '9' {
				return "", errUnsupported
			}
			push(p[s[i]-'1'])
		case 'P', 'g':
			i++
			if i >= len(s) {
				return "", errUnsupported
			}
			slot, isStatic, ok := variable(s[i])
			if !ok {
				return "", errUnsupported
			}
			vars := &dynamic
			if isStatic {
				vars = &static
			}
			if c == 'g' {
				push(vars[slot])
				continue
			}
			n, err := popNumber()
			if err != nil {
				return "", err
			}
			vars[slot] = n
		case '\'':
			if i+2 >= len(s) || s[i+2] != '\'' {
				return "", errUnsupported
			}
			push(int(s[i+1]))
			i += 2
		case '{':
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				return "", errUnsupported
			}
			n, err := strconv.Atoi(s[i+1 : i+end])
			if err != nil {
				return "", errUnsupported
			}
			push(n)
			i += end
		case 'l':
			top, err := pop()
			if err != nil || !top.isString {
				return "", errUnsupported
			}
			push(len(top.text))
		case '+', '-', '*', '/', 'm', '&', '|', '^', '=', '>', '<', 'A', 'O':
			b, err := popNumber()
			if err != nil {
				return "", err
			}
			a, err := popNumber()
			if err != nil {
				return "", err
			}
			push(operate(c, a, b))
		case '!':
			n, err := popNumber()
			if err != nil {
				return "", err
			}
			push(truth(n == 0))
		case '~':
			n, err := popNumber()
			if err != nil {
				return "", err
			}
			push(^n)
		case 'i':
			p[0]++
			p[1]++
		case '?', ';':
		case 't':
			n, err := popNumber()
			if err != nil {
				return "", err
			}
			if n == 0 {
				next, ok := skipTo(s, i+1, true)
				if !ok {
					return "", errUnsupported
				}
				i = next
			}
		case 'e':
			next, ok := skipTo(s, i+1, false)
			if !ok {
				return "", errUnsupported
			}
			i = next
		default:
			end, verb, ok := printfSpec(s, i)
			if !ok {
				return "", errUnsupported
			}
			top, err := pop()
			if err != nil {
				return "", err
			}
			formatted, ok := format(s[i:end+1], verb, top)
			if !ok {
				return "", errUnsupported
			}
			out.WriteString(formatted)
			i = end
		}
	}
	return out.String(), nil
}

// variable reads the letter after %P or %g: a to z is a dynamic variable and
// A to Z a static one.
func variable(letter byte) (slot int, isStatic, ok bool) {
	switch {
	case letter >= 'a' && letter <= 'z':
		return int(letter - 'a'), false, true
	case letter >= 'A' && letter <= 'Z':
		return int(letter - 'A'), true, true
	}
	return 0, false, false
}

// operate applies a two-operand operation in postfix order, a being the
// operand pushed first.
func operate(op byte, a, b int) int {
	switch op {
	case '+':
		return a + b
	case '-':
		return a - b
	case '*':
		return a * b
	case '/':
		if b == 0 {
			return 0
		}
		return a / b
	case 'm':
		if b == 0 {
			return 0
		}
		return a % b
	case '&':
		return a & b
	case '|':
		return a | b
	case '^':
		return a ^ b
	case '=':
		return truth(a == b)
	case '>':
		return truth(a > b)
	case '<':
		return truth(a < b)
	case 'A':
		return truth(a != 0 && b != 0)
	case 'O':
		return truth(a != 0 || b != 0)
	}
	return 0
}

// truth is a comparison's result as the number the language pushes.
func truth(b bool) int {
	if b {
		return 1
	}
	return 0
}

// skipTo moves past the part of a conditional that is not taken. From a
// false %t it stops after the matching %e, so the else part runs, or at the
// matching %;. From %e, reached at the end of a taken then part, it stops at
// the matching %;. Nested conditionals are skipped whole. It returns the
// index of the operation's letter, which the caller's loop steps past.
func skipTo(s string, from int, elseCounts bool) (int, bool) {
	depth := 0
	for i := from; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		i++
		if i >= len(s) {
			return 0, false
		}
		switch s[i] {
		case '?':
			depth++
		case ';':
			if depth == 0 {
				return i, true
			}
			depth--
		case 'e':
			if depth == 0 && elseCounts {
				return i, true
			}
		}
	}
	return 0, false
}

// printfSpec reads a %[[:]flags][width[.precision]][doxXs] operation starting
// at s[i], which is the character after the %. It returns the index of the
// conversion letter and the letter itself.
func printfSpec(s string, i int) (int, byte, bool) {
	j := i
	if j < len(s) && s[j] == ':' {
		j++
	}
	for j < len(s) && strings.IndexByte("-+# ", s[j]) >= 0 {
		j++
	}
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j < len(s) && s[j] == '.' {
		j++
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
	}
	if j >= len(s) || strings.IndexByte("doxXs", s[j]) < 0 {
		return 0, 0, false
	}
	return j, s[j], true
}

// format renders one printf-style operation, spec being its text from the
// character after the % up to and including the conversion letter.
func format(spec string, verb byte, top value) (string, bool) {
	directive := "%" + strings.TrimPrefix(spec, ":")
	if verb == 's' {
		if !top.isString {
			return "", false
		}
		return fmt.Sprintf(directive, top.text), true
	}
	if top.isString {
		return "", false
	}
	return fmt.Sprintf(directive, top.number), true
}
