package main

import "testing"

func TestAuthProgressMessageStageUpdate(t *testing.T) {
	line := `{"event":"auth_stage_update","stage":"otp_input"}`
	got := authProgressMessage(line)
	if got != "Auth stage: otp input" {
		t.Fatalf("authProgressMessage() = %q, want %q", got, "Auth stage: otp input")
	}
}

func TestAuthProgressMessageSecureInputURL(t *testing.T) {
	line := `{"event":"auth_secure_input_url","secure_input_url":"https://example.invalid/token"}`
	got := authProgressMessage(line)
	if got != "Secure input URL sent to Discord." {
		t.Fatalf("authProgressMessage() = %q, want secure-input status", got)
	}
}

func TestAuthProgressMessageSecureInputURLWithExpiry(t *testing.T) {
	line := `{"event":"auth_secure_input_url","secure_input_url":"https://example.invalid/token","secure_input_expires_in_sec":299}`
	got := authProgressMessage(line)
	if got != "Secure input URL sent to Discord (expires in 4m59s)." {
		t.Fatalf("authProgressMessage() = %q, want secure-input expiry status", got)
	}
}

func TestAuthProgressMessageKMSIContinue(t *testing.T) {
	line := `{"event":"auth_kmsi_continue","attempt":1}`
	got := authProgressMessage(line)
	if got != "Handling stay signed in prompt ..." {
		t.Fatalf("authProgressMessage() = %q, want kmsi handling status", got)
	}
}
