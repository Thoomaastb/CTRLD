package mfa

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"bytes"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	// TOTPIssuer ist der Name der App im Authenticator
	TOTPIssuer = "CTRLD"
	// TOTPDigits — 6-stelliger Code (RFC 6238 Standard)
	TOTPDigits = otp.DigitsSix
	// TOTPPeriod — 30 Sekunden (RFC 6238 Standard)
	TOTPPeriod = 30
	// BackupCodeCount — 8 Backup-Codes
	BackupCodeCount = 8
	// BackupCodeLen — 10 Zeichen pro Code
	BackupCodeLen = 10
)

var (
	ErrInvalidTOTPCode  = errors.New("mfa: ungültiger totp code")
	ErrInvalidBackupCode = errors.New("mfa: ungültiger backup code")
	ErrCodeAlreadyUsed  = errors.New("mfa: code bereits verwendet")
)

// TOTPSetup enthält alle Daten für die TOTP-Einrichtung.
type TOTPSetup struct {
	// Secret ist der Base32-kodierte TOTP-Secret
	Secret string
	// QRCodePNG ist der QR-Code als Base64-kodiertes PNG
	QRCodePNG string
	// ManualEntryKey ist der formatierte Key für manuelle Eingabe
	ManualEntryKey string
	// ProvisioningURI ist der otpauth:// URI
	ProvisioningURI string
}

// GenerateTOTPSetup erstellt einen neuen TOTP-Key für einen User.
func GenerateTOTPSetup(email string) (*TOTPSetup, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: email,
		Digits:      TOTPDigits,
		Period:      TOTPPeriod,
		Algorithm:   otp.AlgorithmSHA1, // RFC 6238 Standard — maximale App-Kompatibilität
	})
	if err != nil {
		return nil, fmt.Errorf("mfa: totp key generierung fehlgeschlagen: %w", err)
	}

	// QR-Code als PNG generieren
	img, err := key.Image(256, 256)
	if err != nil {
		return nil, fmt.Errorf("mfa: qr-code generierung fehlgeschlagen: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("mfa: qr-code encoding fehlgeschlagen: %w", err)
	}

	qrBase64 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	// Manueller Entry-Key: Gruppen von 4 Zeichen für Lesbarkeit
	secret := key.Secret()
	manualKey := formatManualKey(secret)

	return &TOTPSetup{
		Secret:          secret,
		QRCodePNG:       qrBase64,
		ManualEntryKey:  manualKey,
		ProvisioningURI: key.URL(),
	}, nil
}

// VerifyTOTP prüft einen TOTP-Code gegen den gespeicherten Secret.
// Erlaubt ein Zeitfenster von ±1 Periode (30s) für Uhrabweichungen.
func VerifyTOTP(secret, code string) (bool, error) {
	// Code-Format validieren
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false, ErrInvalidTOTPCode
	}

	valid, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    TOTPPeriod,
		Skew:      1, // ±1 Periode Toleranz
		Digits:    TOTPDigits,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return false, fmt.Errorf("mfa: totp validierung fehlgeschlagen: %w", err)
	}

	return valid, nil
}

// BackupCode ist ein einmalig verwendbarer Recovery-Code.
type BackupCode struct {
	Code     string `json:"code"`
	UsedAt   string `json:"used_at,omitempty"`
	IsUsed   bool   `json:"is_used"`
}

// GenerateBackupCodes erstellt 8 zufällige Backup-Codes.
// Die Codes werden im Klartext zurückgegeben (nur einmalig sichtbar).
// Gespeichert werden die Codes als JSON mit Used-Status.
func GenerateBackupCodes() ([]BackupCode, error) {
	codes := make([]BackupCode, BackupCodeCount)

	for i := range codes {
		code, err := generateRandomCode(BackupCodeLen)
		if err != nil {
			return nil, fmt.Errorf("mfa: backup code generierung fehlgeschlagen: %w", err)
		}
		codes[i] = BackupCode{
			Code:   code,
			IsUsed: false,
		}
	}

	return codes, nil
}

// SerializeBackupCodes serialisiert die Codes für DB-Speicherung.
func SerializeBackupCodes(codes []BackupCode) (string, error) {
	data, err := json.Marshal(codes)
	if err != nil {
		return "", fmt.Errorf("mfa: backup codes serialisierung fehlgeschlagen: %w", err)
	}
	return string(data), nil
}

// DeserializeBackupCodes deserialisiert Backup-Codes aus der DB.
func DeserializeBackupCodes(data string) ([]BackupCode, error) {
	var codes []BackupCode
	if err := json.Unmarshal([]byte(data), &codes); err != nil {
		return nil, fmt.Errorf("mfa: backup codes deserialisierung fehlgeschlagen: %w", err)
	}
	return codes, nil
}

// UseBackupCode prüft einen Backup-Code und markiert ihn als verwendet.
// Gibt die aktualisierten Codes zurück für DB-Update.
func UseBackupCode(codes []BackupCode, inputCode string) ([]BackupCode, error) {
	inputCode = strings.ToUpper(strings.TrimSpace(inputCode))

	for i, code := range codes {
		if code.IsUsed {
			continue
		}
		if strings.EqualFold(code.Code, inputCode) {
			codes[i].IsUsed = true
			codes[i].UsedAt = time.Now().UTC().Format(time.RFC3339)
			return codes, nil
		}
	}

	return nil, ErrInvalidBackupCode
}

// generateRandomCode erstellt einen zufälligen alphanumerischen Code.
func generateRandomCode(length int) (string, error) {
	// Nur Großbuchstaben und Ziffern, ohne verwechselbare Zeichen (0, O, I, L)
	const charset = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	for i, b := range bytes {
		bytes[i] = charset[int(b)%len(charset)]
	}

	// Format: XXXXX-XXXXX
	code := string(bytes)
	if length >= 10 {
		return code[:5] + "-" + code[5:], nil
	}
	return code, nil
}

// formatManualKey formatiert einen Base32-Secret für manuelle Eingabe.
// Beispiel: JBSWY3DPEHPK3PXP → JBSWY 3DPEH PK3PX P
func formatManualKey(secret string) string {
	secret = strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	// Padding entfernen
	secret = strings.TrimRight(secret, "=")

	var groups []string
	for i := 0; i < len(secret); i += 4 {
		end := i + 4
		if end > len(secret) {
			end = len(secret)
		}
		groups = append(groups, secret[i:end])
	}
	return strings.Join(groups, " ")
}

// EncryptTOTPSecret verschlüsselt den TOTP-Secret für die DB.
// In v1.x: Base64-Encoding (kein echtes Encrypt — wird in v1.x+ durch AES-GCM ersetzt)
// TODO v1.x: AES-256-GCM mit Key Derivation aus Master-Secret
func EncryptTOTPSecret(secret string) (string, error) {
	_ = base32.StdEncoding // Import sicherstellen
	return base64.StdEncoding.EncodeToString([]byte(secret)), nil
}

// DecryptTOTPSecret dekodiert den TOTP-Secret aus der DB.
func DecryptTOTPSecret(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("mfa: secret dekodierung fehlgeschlagen: %w", err)
	}
	return string(data), nil
}
