// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package helpers_test

import (
	"testing"

	"template/pkg/helpers"
)

func TestIsArrayContains(t *testing.T) {
	// тест тохиолдол 1
	arr := []string{"hello", "world", "golang"}
	str := "golang"
	expected := true
	result := helpers.IsArrayContains(arr, str)
	if result != expected {
		t.Errorf("Expected %t but got %t", expected, result)
	}

	// тест тохиолдол 2
	arr = []string{"hello", "world", "golang"}
	str = "java"
	expected = false
	result = helpers.IsArrayContains(arr, str)
	if result != expected {
		t.Errorf("Expected %t but got %t", expected, result)
	}
}
