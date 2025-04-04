package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStruct is a test structure used for testing the GetTypeName function
type TestStruct struct{}

// TestMethod is a method on TestStruct used for testing the GetTypeName function
func (t *TestStruct) TestMethod() {}

// Regular function for testing
func testFunction() {}

func TestGetTypeName_MethodOnStruct(t *testing.T) {
	result := GetTypeName((*TestStruct).TestMethod)
	assert.Contains(t, result, "TestStruct", "Type name should contain TestStruct")
}

func TestGetTypeName_MethodOnStructInstance(t *testing.T) {
	result := GetTypeName((&TestStruct{}).TestMethod)
	assert.Contains(t, result, "TestStruct", "Type name should contain TestStruct")
}

func TestGetTypeName_RegularFunction(t *testing.T) {
	result := GetTypeName(testFunction)
	assert.Contains(t, result, "utils", "Type name for regular function should contain package name")
}

// TestStruct2 is a second test structure to avoid interference with GetTypeName tests
type TestStruct2 struct{}

// TestMethod is a method on TestStruct2 that calls GetCurrentTypeName
func (t *TestStruct2) TestMethod() string {
	return GetCurrentTypeName()
}

// Function that calls GetCurrentTypeName
func getCurrentTypeNameWrapper() string {
	return GetCurrentTypeName()
}

func TestGetCurrentTypeName_MethodOnStruct(t *testing.T) {
	// Test with a method on a struct
	result := (&TestStruct2{}).TestMethod()
	assert.Equal(t, "TestStruct2", result, "Current type name should be TestStruct2")
}

func TestGetCurrentTypeName_RegularFunction(t *testing.T) {
	// Test with a regular function
	result := getCurrentTypeNameWrapper()
	assert.True(t, strings.HasSuffix(result, "utils"),
		"Expected result '%s' to end with 'utils'", result)
}

func TestGetCurrentTypeName_DirectCall(t *testing.T) {
	// Test direct call within the test function
	result := GetCurrentTypeName()
	assert.True(t, strings.HasSuffix(result, "utils"),
		"Expected result '%s' to end with 'utils'", result)
}
