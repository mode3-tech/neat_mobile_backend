package errors

import (
	"encoding/json"
	"testing"
)

func TestAlatError_UnmarshalJSON_NumbersAndStrings(t *testing.T) {
	// ALAT sends numeric statusCode and code
	jsonNumeric := []byte(`{"statusCode": 422, "code": 1, "message": "Invalid BVN provided", "errors": ["BVN not found"]}`)
	var errNum AlatError
	if err := json.Unmarshal(jsonNumeric, &errNum); err != nil {
		t.Fatalf("failed to unmarshal numeric AlatError: %v", err)
	}
	if errNum.StatusCode != "422" {
		t.Fatalf("expected StatusCode '422', got %q", errNum.StatusCode)
	}
	if errNum.Code != "1" {
		t.Fatalf("expected Code '1', got %q", errNum.Code)
	}
	if errNum.Message != "Invalid BVN provided" {
		t.Fatalf("unexpected message: %q", errNum.Message)
	}

	// ALAT sends string code
	jsonString := []byte(`{"statusCode": 400, "code": "InvalidBvn", "message": "Failed", "errors": []}`)
	var errStr AlatError
	if err := json.Unmarshal(jsonString, &errStr); err != nil {
		t.Fatalf("failed to unmarshal string AlatError: %v", err)
	}
	if errStr.StatusCode != "400" {
		t.Fatalf("expected StatusCode '400', got %q", errStr.StatusCode)
	}
	if errStr.Code != "InvalidBvn" {
		t.Fatalf("expected Code 'InvalidBvn', got %q", errStr.Code)
	}
}

