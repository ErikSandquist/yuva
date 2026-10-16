package api

import (
	"testing"

	"uuid"
)

func TestPrincipalCreator(t *testing.T) {
	if (principal{}).creator() != nil {
		t.Fatal("a principal without a member or key must not be recorded as a creator")
	}
	m := uuid.New()
	if got := (principal{memberID: m}).creator(); got == nil || *got != m {
		t.Fatalf("member creator %v", got)
	}
}
