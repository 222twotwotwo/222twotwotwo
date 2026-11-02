package mapreduce

import (
	"strconv"
	"strings"
	"unicode"
)

// WordCountMap treats each consecutive sequence of Unicode letters as a word.
// Word matching is case-sensitive.
func WordCountMap(_ string, contents string) []KeyValue {
	words := strings.FieldsFunc(contents, func(r rune) bool { return !unicode.IsLetter(r) })
	result := make([]KeyValue, len(words))
	for i, word := range words {
		result[i] = KeyValue{Key: word, Value: "1"}
	}
	return result
}

func WordCountReduce(_ string, values []string) string {
	return strconv.Itoa(len(values))
}
