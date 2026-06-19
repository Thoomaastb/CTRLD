package mfa

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/auth"
	database "github.com/Thoomaastb/CTRLD/internal/db"
	db "github.com/Thoomaastb/CTRLD/internal/db/generated"
)

var (
	ErrMFAAlreadySetup   = errors.New("mfa: totp bereits eingerichtet")
	ErrMFANotSetup       = errors.New("mfa: kein mfa eingerichtet")
	ErrVerificationFailed = errors.New("mfa: verifikation fehlgeschlagen")
)

// Service kapselt die MFA-Logik.
type Service struct {
	database *database.DB
	queries  *db.Queries
	tokenCfg auth.TokenConfig
	log      zerolog.Logger
}

// NewService erstellt einen neuen MFA-Service.
func NewService(d *database.DB, tokenCfg auth.TokenConfig, log zerolog.Logger) *Service {
	return &Service{
		database: d,
		queries:  db.New(d.SQL()),
		tokenCfg: tokenCfg,
		log:      log,
	}
}

// SetupTOTPResponse enthält die Daten für den Setup-Flow.
type SetupTOTPResponse struct {
	Secret         string `json:"secret"`
	QRCodePNG      string `json:"qr_code"`
	ManualEntryKey string `json:"manual_entry_key"`
}

// InitiateTOTPSetup erstellt einen neuen TOTP-Key und gibt Setup-Daten zurück.
// Noch nicht in DB gespeichert — erst nach Bestätigung (ConfirmTOTPSetup).
func (s *Service) InitiateTOTPSetup(ctx context.Context, userID string) (*SetupTOTPResponse, error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("mfa: user nicht gefunden: %w", err)
	}

	setup, err := GenerateTOTPSetup(user.Email)
	if err != nil {
		return nil, err
	}

	return &SetupTOTPResponse{
		Secret:         setup.Secret,
		QRCodePNG:      setup.QRCodePNG,
		ManualEntryKey: setup.ManualEntryKey,
	}, nil
}

// ConfirmTOTPSetupParams enthält die Parameter für die TOTP-Bestätigung.
type ConfirmTOTPSetupParams struct {
	UserID      string
	Secret      string // Der Secret aus InitiateTOTPSetup
	Code        string // Der erste Code aus der Authenticator-App
	DeviceName  string // z.B. "iPhone 15" oder "Google Authenticator"
}

// ConfirmTOTPSetupResult enthält Backup-Codes nach erfolgreicher Einrichtung.
type ConfirmTOTPSetupResult struct {
	BackupCodes []string // Klartext — nur einmalig anzeigen!
	CredentialID string
}

// ConfirmTOTPSetup bestätigt die TOTP-Einrichtung und speichert in der DB.
// Der User muss einen gültigen Code eingeben um die Einrichtung zu bestätigen.
func (s *Service) ConfirmTOTPSetup(ctx context.Context, params ConfirmTOTPSetupParams) (*ConfirmTOTPSetupResult, error) {
	// Code verifizieren
	ok, err := VerifyTOTP(params.Secret, params.Code)
	if err != nil || !ok {
		return nil, ErrVerificationFailed
	}

	// Secret verschlüsseln für DB-Speicherung
	encrypted, err := EncryptTOTPSecret(params.Secret)
	if err != nil {
		return nil, err
	}

	// Backup-Codes generieren
	backupCodes, err := GenerateBackupCodes()
	if err != nil {
		return nil, err
	}

	// In DB speichern
	credID := uuid.New().String()
	_, err = s.queries.CreateMFACredential(ctx, db.CreateMFACredentialParams{
		ID:             credID,
		UserID:         params.UserID,
		Type:           "totp",
		Name:           params.DeviceName,
		CredentialData: encrypted,
	})
	if err != nil {
		return nil, fmt.Errorf("mfa: credential speichern fehlgeschlagen: %w", err)
	}

	// Backup-Codes serialisiert im User-Eintrag speichern
	serialized, err := SerializeBackupCodes(backupCodes)
	if err != nil {
		return nil, err
	}

	_, err = s.database.SQL().ExecContext(ctx,
		"UPDATE users SET backup_codes = ? WHERE id = ?",
		serialized, params.UserID,
	)
	if err != nil {
		return nil, fmt.Errorf("mfa: backup codes speichern fehlgeschlagen: %w", err)
	}

	// Klartext-Codes für einmalige Anzeige
	plainCodes := make([]string, len(backupCodes))
	for i, c := range backupCodes {
		plainCodes[i] = c.Code
	}

	s.logAudit(ctx, params.UserID, "mfa.totp.setup", "success", "info")
	s.log.Info().Str("user_id", params.UserID).Msg("totp eingerichtet")

	return &ConfirmTOTPSetupResult{
		BackupCodes:  plainCodes,
		CredentialID: credID,
	}, nil
}

// VerifyTOTPForLogin verifiziert einen TOTP-Code im Login-Flow.
// Gibt bei Erfolg das vollwertige Token-Paar zurück.
func (s *Service) VerifyTOTPForLogin(ctx context.Context, userID, code, ip, userAgent string) (*auth.Claims, error) {
	// Alle aktiven TOTP-Credentials des Users laden
	creds, err := s.queries.ListMFACredentialsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("mfa: credentials laden fehlgeschlagen: %w", err)
	}

	// Mindestens ein TOTP-Credential muss vorhanden sein
	var totpCreds []db.MfaCredential
	for _, c := range creds {
		if c.Type == "totp" {
			totpCreds = append(totpCreds, c)
		}
	}

	if len(totpCreds) == 0 {
		return nil, ErrMFANotSetup
	}

	// Code gegen alle TOTP-Credentials prüfen
	for _, cred := range totpCreds {
		secret, err := DecryptTOTPSecret(cred.CredentialData)
		if err != nil {
			continue
		}

		ok, err := VerifyTOTP(secret, code)
		if err != nil || !ok {
			continue
		}

		// Sign-Count für TOTP nicht relevant, aber last_used_at aktualisieren
		_ = s.queries.UpdateMFASignCount(ctx, db.UpdateMFASignCountParams{
			SignCount: cred.SignCount,
			ID:        cred.ID,
		})

		s.logAudit(ctx, userID, "auth.mfa.success", "success", "info")

		// User-Daten für Claims laden
		user, err := s.queries.GetUserByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("mfa: user laden fehlgeschlagen: %w", err)
		}

		return &auth.Claims{
			UserID: userID,
			Email:  user.Email,
			Role:   user.Role,
		}, nil
	}

	s.logAudit(ctx, userID, "auth.mfa.failure", "failure", "warning")
	return nil, ErrInvalidTOTPCode
}

// UseBackupCodeForLogin verifiziert einen Backup-Code im Login-Flow.
func (s *Service) UseBackupCodeForLogin(ctx context.Context, userID, code string) (*auth.Claims, error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("mfa: user nicht gefunden: %w", err)
	}

	if !user.BackupCodes.Valid || user.BackupCodes.String == "" {
		return nil, ErrMFANotSetup
	}

	codes, err := DeserializeBackupCodes(user.BackupCodes.String)
	if err != nil {
		return nil, err
	}

	updated, err := UseBackupCode(codes, code)
	if err != nil {
		s.logAudit(ctx, userID, "auth.backup_code.failure", "failure", "warning")
		return nil, err
	}

	// Aktualisierte Codes zurückschreiben
	serialized, _ := SerializeBackupCodes(updated)
	_, _ = s.database.SQL().ExecContext(ctx,
		"UPDATE users SET backup_codes = ? WHERE id = ?",
		serialized, userID,
	)

	s.logAudit(ctx, userID, "auth.backup_code.used", "success", "warning")

	return &auth.Claims{
		UserID: userID,
		Email:  user.Email,
		Role:   user.Role,
	}, nil
}

// ListCredentials gibt alle MFA-Credentials eines Users zurück (ohne Secrets).
func (s *Service) ListCredentials(ctx context.Context, userID string) ([]db.MfaCredential, error) {
	return s.queries.ListMFACredentialsByUserID(ctx, userID)
}

// RemoveCredential entfernt eine MFA-Methode.
func (s *Service) RemoveCredential(ctx context.Context, credID, userID string) error {
	err := s.queries.DeactivateMFACredential(ctx, db.DeactivateMFACredentialParams{
		ID:     credID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("mfa: credential entfernen fehlgeschlagen: %w", err)
	}

	s.logAudit(ctx, userID, "mfa.credential.removed", credID, "warning")
	return nil
}

// TokenSecret gibt das JWT-Secret zurück (für MFA-Handler).
func (s *Service) TokenSecret() []byte {
	return s.tokenCfg.Secret
}

// logAudit schreibt einen Audit-Log-Eintrag.
func (s *Service) logAudit(ctx context.Context, userID, actionType, result, severity string) {
	userIDSQL := sql.NullString{String: userID, Valid: userID != ""}
	_, err := s.queries.CreateAuditEntry(ctx, db.CreateAuditEntryParams{
		ID:         uuid.New().String(),
		UserID:     userIDSQL,
		ActionType: actionType,
		Result:     result,
		Severity:   severity,
	})
	if err != nil {
		s.log.Error().Err(err).Str("action", actionType).Msg("audit log fehlgeschlagen")
	}
}
