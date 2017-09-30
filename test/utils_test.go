package utils_test

import (
	"testing"

	"../utils"
)

func TestSomething(t *testing.T) {
	valToIncrement := 1
	utils.IncIntPtr(&valToIncrement)
	if valToIncrement != 2 {
		t.Error("value to increment not incremented by 1")
	}
}
