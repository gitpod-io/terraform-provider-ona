package tokenproof

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAccessTokenHash(t *testing.T) {
	t.Parallel()

	got := AccessTokenHash("token")
	want := "PEaenWxYddN6Q_NT1PiOYfz4EsZu7jRXRlpAsNpBU-A"

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("AccessTokenHash() mismatch (-want +got):\n%s", diff)
	}
}

func TestCanonicalString(t *testing.T) {
	t.Parallel()

	proof := Proof{
		Version:         VersionNaClBoxDPoP,
		AccessTokenHash: "token-hash",
		RPC:             "/gitpod.v1.RunnerInteractionService/ListRunnerAgentExecutions",
		IssuedAt:        1735689600,
		ID:              "proof-id",
	}

	got := CanonicalString(proof)
	want := "gitpod-cnf-proof-v1\ntoken-hash\n/gitpod.v1.RunnerInteractionService/ListRunnerAgentExecutions\n1735689600\nproof-id"

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("CanonicalString() mismatch (-want +got):\n%s", diff)
	}
}

func TestEncodeDecodeProof(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Proof Proof
		Err   string
	}

	baseProof := Proof{
		Version:         VersionNaClBoxDPoP,
		IssuedAt:        1735689600,
		ID:              "proof-id",
		RPC:             "/gitpod.v1.IdentityService/GetAuthenticatedIdentity",
		AccessTokenHash: "token-hash",
		MAC:             "mac-value",
	}

	tests := []struct {
		Name     string
		Encoded  string
		Setup    func(t *testing.T) string
		Expected Expectation
	}{
		{
			Name: "round_trip",
			Setup: func(t *testing.T) string {
				t.Helper()

				encoded, err := EncodeProof(baseProof)
				if err != nil {
					t.Fatalf("EncodeProof() unexpected error: %v", err)
				}
				return encoded
			},
			Expected: Expectation{
				Proof: baseProof,
			},
		},
		{
			Name:    "empty_header",
			Encoded: "   ",
			Expected: Expectation{
				Err: "proof header is empty",
			},
		},
		{
			Name:    "invalid_base64",
			Encoded: "%%%",
			Expected: Expectation{
				Err: "decode proof header: illegal base64 data at input byte 0",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			header := tc.Encoded
			if tc.Setup != nil {
				header = tc.Setup(t)
			}

			var got Expectation
			proof, err := DecodeProof(header)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Proof = proof
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("DecodeProof() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
