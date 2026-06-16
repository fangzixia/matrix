package run

import "testing"

func TestEncodePipelineStagesEmpty(t *testing.T) {
	if got := encodePipelineStages(nil); got != "[]" {
		t.Fatalf("encode empty = %q, want []", got)
	}
}

func TestDecodePipelineStagesEmpty(t *testing.T) {
	if got := decodePipelineStages(""); got != nil {
		t.Fatalf("decode empty = %v, want nil", got)
	}
	if got := decodePipelineStages("[]"); len(got) != 0 {
		t.Fatalf("decode [] = %v, want empty slice", got)
	}
}
