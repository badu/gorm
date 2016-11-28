package quote

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

var (
	regExpPeriod = regexp.MustCompile("\\.")
)

//original : 11.919s, 11.871s, 11.923s, 11.988s, 12.031s
func Quote(field string) string {
	if strings.Index(field, ".") != -1 {
		newStrs := []string{}
		for _, str := range strings.Split(field, ".") {
			newStrs = append(newStrs, fmt.Sprintf("`%s`", str))
		}
		return strings.Join(newStrs, ".")
	}
	return fmt.Sprintf("`%s`", field)
}

// - with MustCompile in func : 97.937s, 97.362s ... slow as Hell
// - with "prepared" var : 9.022s, 9.054s, 9.026s, 9.055s, 9.137s
func RegexpQuote(field string) string {
	return fmt.Sprintf("`%s`", regExpPeriod.ReplaceAllString(field, "`.`"))
}

//12.963s, 12.550s, 12.719s, 12.859s, 12.613s
func QuoteWithRunes(field string) string {
	result := ""
	//rune is a alias for int32 as a Unicode character can be 1, 2 or 4 bytes in UTF-8 encoding
	for i, w := 0, 0; i < len(field); i += w {
		runeValue, width := utf8.DecodeRuneInString(field[i:])
		if runeValue == '.' {
			//period detected
			result += "`.`"
		} else {
			result += string(runeValue)
		}
		w = width
	}
	return fmt.Sprintf("`%s`", result)
}

//13.488s, 13.433s, 13.425s, 13.398s, 13.309s
func QuoteWithRunesConv(field string) string {
	result := ""
	runes := []rune(field)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '.' {
			result += "`.`"
		} else {
			result += string(runes[i])
		}
	}
	return fmt.Sprintf("`%s`", result)
}

//15.196s, 15.154s, 15.220s, 15.014s, 15.073s
func FastQuote(field string) string {
	result := ""
	work := []byte(field)
	for utf8.RuneCount(work) > 1 {
		r, size := utf8.DecodeRune(work)
		work = work[size:]
		if r == '.' {
			//period detected
			result += "`.`"
		} else {
			result += string(r)
		}
	}
	return fmt.Sprintf("`%s`", result)
}

func BenchmarkQuote(t *testing.B) {
	fieldName := "abra.ca.dabra.白鵬翔"
	i := 10000000
	var lastResult string
	for i > 0 {
		result := Quote(fieldName)
		i--
		if i == 0 {
			lastResult = result
		}
	}
	t.Logf("Finished BenchmarkQuote : %q", lastResult)
}

func BenchmarkRegExpQuote(t *testing.B) {
	fieldName := "abra.ca.dabra.白鵬翔"
	i := 10000000
	var lastResult string
	for i > 0 {
		result := RegexpQuote(fieldName)
		i--
		if i == 0 {
			lastResult = result
		}
	}
	t.Logf("Finished BenchmarkRegExpQuote : %q", lastResult)
}

func BenchmarkQuoteWithRunes(t *testing.B) {
	fieldName := "abra.ca.dabra.白鵬翔"
	i := 10000000
	var lastResult string
	for i > 0 {
		result := QuoteWithRunes(fieldName)
		i--
		if i == 0 {
			lastResult = result
		}
	}
	t.Logf("Finished BenchmarkQuoteWithRunes : %q", lastResult)
}

func BenchmarkQuoteWithRunesConv(t *testing.B) {
	fieldName := "abra.ca.dabra.白鵬翔"
	i := 10000000
	var lastResult string
	for i > 0 {
		result := QuoteWithRunesConv(fieldName)
		i--
		if i == 0 {
			lastResult = result
		}
	}
	t.Logf("Finished BenchmarkQuoteWithRunes : %q", lastResult)
}

func BenchmarkFastQuote(t *testing.B) {
	fieldName := "abra.ca.dabra.白鵬翔"
	i := 10000000
	var lastResult string
	for i > 0 {
		result := FastQuote(fieldName)
		i--
		if i == 0 {
			lastResult = result
		}
	}
	t.Logf("Finished BenchmarkFastQuote : %q", lastResult)
}
