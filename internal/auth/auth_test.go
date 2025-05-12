package agent

import (
	"testing"
)

func TestCompute(t *testing.T) {
	compute := TestExport.Compute

	testCases := []struct {
		name     string
		arg1     float64
		arg2     float64
		op       string
		expected float64
	}{
		{"Addition", 5.0, 3.0, "+", 8.0},
		{"Subtraction", 5.0, 3.0, "-", 2.0},
		{"Multiplication", 5.0, 3.0, "*", 15.0},
		{"Division", 15.0, 3.0, "/", 5.0},
		{"Division by zero", 5.0, 0.0, "/", 0.0},
		{"Invalid operation", 5.0, 3.0, "?", 0.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := compute(tc.arg1, tc.arg2, tc.op)
			if result != tc.expected {
				t.Errorf("%s: Expected %f, got %f", tc.name, tc.expected, result)
			}
		})
	}
}

type TestExporter struct {
	Compute func(arg1, arg2 float64, op string) float64
}

var TestExport = TestExporter{
	Compute: compute,
}
