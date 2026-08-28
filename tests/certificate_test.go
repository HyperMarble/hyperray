package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/HyperMarble/ray/internal/certificate"
)

func TestCertificateRejectsIncompleteEvidence(t *testing.T) {
	if _, err := certificate.Issue(certificate.Document{}); err == nil {
		t.Fatal("empty certificate evidence was accepted")
	}
}

func TestCertificateRejectsUnknownAndNonCanonicalJSON(t *testing.T) {
	unknown := []byte(`{"schema":"ray.verification-certificate/v3","unknown":true}`)
	if _, err := certificate.VerifyBytes(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown certificate field was not rejected: %v", err)
	}
	encoded, err := json.Marshal(certificate.Certificate{})
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if _, err := certificate.VerifyBytes(encoded); err == nil {
		t.Fatal("invalid/noncanonical certificate bytes were accepted")
	}
}
