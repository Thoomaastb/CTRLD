package mfa_test

import (
	"strings"
	"testing"

	"github.com/Thoomaastb/CTRLD/internal/mfa"
)

func TestGenerateTOTPSetup(t *testing.T) {
	setup, err := mfa.GenerateTOTPSetup("admin@example.com")
	if err != nil {
		t.Fatalf("setup fehlgeschlagen: %v", err)
	}

	if setup.Secret == "" {
		t.Error("secret ist leer")
	}
	if !strings.HasPrefix(setup.QRCodePNG, "data:image/png;base64,") {
		t.Error("qr-code hat falsches format")
	}
	if setup.ManualEntryKey == "" {
		t.Error("manual entry key ist leer")
	}
	if !strings.HasPrefix(setup.ProvisioningURI, "otpauth://totp/") {
		t.Error("provisioning uri hat falsches format")
	}
}

func TestGenerateTOTPSetup_ContainsIssuer(t *testing.T) {
	setup, err := mfa.GenerateTOTPSetup("user@test.com")
	if err != nil {
		t.Fatalf("setup fehlgeschlagen: %v", err)
	}

	if !strings.Contains(setup.ProvisioningURI, "CTRLD") {
		t.Error("provisioning uri sollte issuer 'CTRLD' enthalten")
	}
	if !strings.Contains(setup.ProvisioningURI, "user@test.com") {
		t.Error("provisioning uri sollte email enthalten")
	}
}

func TestVerifyTOTP_InvalidCode_Length(t *testing.T) {
	setup, _ := mfa.GenerateTOTPSetup("test@example.com")

	// Zu kurz
	ok, err := mfa.VerifyTOTP(setup.Secret, "12345")
	if err == nil && ok {
		t.Error("zu kurzer code sollte abgelehnt werden")
	}

	// Zu lang
	ok, err = mfa.VerifyTOTP(setup.Secret, "1234567")
	if err == nil && ok {
		t.Error("zu langer code sollte abgelehnt werden")
	}
}

func TestVerifyTOTP_WrongCode(t *testing.T) {
	setup, _ := mfa.GenerateTOTPSetup("test@example.com")

	ok, _ := mfa.VerifyTOTP(setup.Secret, "000000")
	if ok {
		t.Error("falscher code sollte abgelehnt werden")
	}
}

func TestGenerateBackupCodes_Count(t *testing.T) {
	codes, err := mfa.GenerateBackupCodes()
	if err != nil {
		t.Fatalf("backup codes fehlgeschlagen: %v", err)
	}

	if len(codes) != mfa.BackupCodeCount {
		t.Errorf("erwartet %d codes, bekommen %d", mfa.BackupCodeCount, len(codes))
	}
}

func TestGenerateBackupCodes_Unique(t *testing.T) {
	codes, _ := mfa.GenerateBackupCodes()

	seen := make(map[string]bool)
	for _, code := range codes {
		if seen[code.Code] {
			t.Errorf("doppelter backup code: %s", code.Code)
		}
		seen[code.Code] = true
	}
}

func TestGenerateBackupCodes_NotUsed(t *testing.T) {
	codes, _ := mfa.GenerateBackupCodes()

	for _, code := range codes {
		if code.IsUsed {
			t.Error("neue codes sollten nicht als verwendet markiert sein")
		}
		if code.Code == "" {
			t.Error("code darf nicht leer sein")
		}
	}
}

func TestSerializeDeserializeBackupCodes(t *testing.T) {
	original, _ := mfa.GenerateBackupCodes()

	serialized, err := mfa.SerializeBackupCodes(original)
	if err != nil {
		t.Fatalf("serialisierung fehlgeschlagen: %v", err)
	}

	restored, err := mfa.DeserializeBackupCodes(serialized)
	if err != nil {
		t.Fatalf("deserialisierung fehlgeschlagen: %v", err)
	}

	if len(restored) != len(original) {
		t.Errorf("anzahl codes: erwartet %d, bekommen %d", len(original), len(restored))
	}

	for i, code := range original {
		if code.Code != restored[i].Code {
			t.Errorf("code[%d] unterschiedlich: %s vs %s", i, code.Code, restored[i].Code)
		}
	}
}

func TestUseBackupCode_Valid(t *testing.T) {
	codes, _ := mfa.GenerateBackupCodes()
	firstCode := codes[0].Code

	updated, err := mfa.UseBackupCode(codes, firstCode)
	if err != nil {
		t.Fatalf("backup code verwenden fehlgeschlagen: %v", err)
	}

	if !updated[0].IsUsed {
		t.Error("code sollte als verwendet markiert sein")
	}
	if updated[0].UsedAt == "" {
		t.Error("used_at sollte gesetzt sein")
	}
}

func TestUseBackupCode_Invalid(t *testing.T) {
	codes, _ := mfa.GenerateBackupCodes()

	_, err := mfa.UseBackupCode(codes, "XXXXX-XXXXX")
	if err == nil {
		t.Error("ungültiger code sollte fehler ergeben")
	}
}

func TestUseBackupCode_AlreadyUsed(t *testing.T) {
	codes, _ := mfa.GenerateBackupCodes()
	firstCode := codes[0].Code

	// Ersten Mal verwenden
	codes, _ = mfa.UseBackupCode(codes, firstCode)

	// Zweites Mal sollte fehlschlagen
	_, err := mfa.UseBackupCode(codes, firstCode)
	if err == nil {
		t.Error("bereits verwendeter code sollte abgelehnt werden")
	}
}

func TestEncryptDecryptTOTPSecret(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"

	encrypted, err := mfa.EncryptTOTPSecret(secret)
	if err != nil {
		t.Fatalf("verschlüsselung fehlgeschlagen: %v", err)
	}

	decrypted, err := mfa.DecryptTOTPSecret(encrypted)
	if err != nil {
		t.Fatalf("entschlüsselung fehlgeschlagen: %v", err)
	}

	if decrypted != secret {
		t.Errorf("erwartet %q, bekommen %q", secret, decrypted)
	}
}
