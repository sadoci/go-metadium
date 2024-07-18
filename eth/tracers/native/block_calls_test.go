package native // replace with your actual package name

import (
	"bytes"
	"fmt"
	"testing"
)

func TestGetResult(t *testing.T) {
	t.Run("test case 1", func(t *testing.T) {
		tracer := &blockCallsTracer{
			// initialize fields as necessary for the test
		}
		var callstack []callFrame
		callstack = append(callstack, callFrame{})
		callstack = append(callstack, callFrame{})
		tracer.callstacks = append(tracer.callstacks, callstack)
		tracer.callstacks = append(tracer.callstacks, callstack)

		result, err := tracer.GetResult()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		got, err := result.MarshalJSON()
		if err != nil {
			t.Fatalf("marshal failed %v", err)
		}

		fmt.Printf("%v\n", string(got))

		expected := "abc"
		if !bytes.Equal(got, []byte(expected)) {
			t.Errorf("expected %v, got %v", expected, string(result))
		}
	})

	// Add more test cases as needed
}
