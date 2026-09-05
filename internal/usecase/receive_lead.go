package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/id"
	"github.com/lunyashon/sdmp/internal/port"
)

const (
	bitrixUserID         = "1"
	idempotencyLeadField = "lead_id"
	idempotencyTTL       = 24 * time.Hour
)

type ReceiveLeadInput struct {
	UserID     string
	Token      string
	Phone      string
	SourceCode string
	WebID      string
	Payload    map[string]any
}

type ReceiveLeadService struct {
	sources port.SourceRepository
	leads   port.LeadRepository
	storage port.S3
	cache   port.Redis
	log     *slog.Logger
	now     func() time.Time
	uuid    func() (string, error)
}

func NewReceiveLeadService(
	sources port.SourceRepository,
	leads port.LeadRepository,
	storage port.S3,
	cache port.Redis,
	log *slog.Logger,
) *ReceiveLeadService {
	if log == nil {
		log = slog.Default()
	}
	return &ReceiveLeadService{
		sources: sources,
		leads:   leads,
		storage: storage,
		cache:   cache,
		log:     log,
		now:     time.Now,
		uuid:    id.NewUUID,
	}
}

func (s *ReceiveLeadService) Receive(ctx context.Context, in ReceiveLeadInput) (int64, error) {
	if strings.TrimSpace(in.UserID) != bitrixUserID {
		return 0, domain.ErrUnauthorized
	}
	token := strings.TrimSpace(in.Token)
	if token == "" {
		return 0, domain.ErrUnauthorized
	}

	src, err := s.sources.GetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return 0, domain.ErrUnauthorized
		}
		return 0, err
	}
	if !src.IsActive {
		return 0, domain.ErrForbidden
	}

	phone := strings.TrimSpace(in.Phone)
	if phone == "" {
		return 0, fmt.Errorf("%w: PHONE is required", domain.ErrInvalidInput)
	}
	sourceCode := strings.TrimSpace(in.SourceCode)
	if sourceCode != "" && src.BitrixCode != "" && sourceCode != src.BitrixCode {
		return 0, fmt.Errorf("%w: SOURCE_ID mismatch", domain.ErrInvalidInput)
	}
	webID := strings.TrimSpace(in.WebID)

	if src.IdempotencyActive && webID != "" {
		existing, ok, err := s.lookupIdempotency(ctx, src.ID, webID)
		if err != nil {
			return 0, fmt.Errorf("idempotency lookup: %w", err)
		}
		if ok {
			s.log.InfoContext(ctx, "lead idempotent hit",
				"lead_id", existing,
				"source_id", src.ID,
			)
			return existing, nil
		}
	}

	uid, err := s.uuid()
	if err != nil {
		return 0, fmt.Errorf("uuid: %w", err)
	}
	createdAt := s.now()
	s3Key := rawObjectKey(src.ID, createdAt, uid)

	payload := in.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("%w: marshal payload: %v", domain.ErrInvalidInput, err)
	}

	if err := s.storage.Put(ctx, s3Key, bytes.NewReader(raw), port.PutOptions{
		ContentType: "application/json",
	}); err != nil {
		return 0, fmt.Errorf("s3 put: %w", err)
	}

	leadID, err := s.leads.InsertAccepted(ctx, entity.LeadRow{
		UUID:      uid,
		Phone:     phone,
		SourceID:  src.ID,
		WebID:     webID,
		StatusID:  entity.StatusAccepted,
		S3Key:     s3Key,
		CreatedAt: createdAt,
	}, entity.OutboxMessage{
		SourceID:  src.ID,
		Payload:   string(raw),
		CreatedAt: createdAt,
	})
	if err != nil {
		if delErr := s.storage.Delete(ctx, s3Key); delErr != nil {
			s.log.ErrorContext(ctx, "s3 cleanup after db error", "key", s3Key, "error", delErr)
		}
		return 0, err
	}

	if src.IdempotencyActive && webID != "" {
		if err := s.storeIdempotency(ctx, src.ID, webID, leadID); err != nil {
			s.log.ErrorContext(ctx, "idempotency store failed",
				"lead_id", leadID,
				"source_id", src.ID,
				"error", err,
			)
		}
	}

	s.log.InfoContext(ctx, "lead accepted",
		"lead_id", leadID,
		"source_id", src.ID,
		"s3_key", s3Key,
	)
	return leadID, nil
}

func idempotencyKey(sourceID int, webID string) string {
	return fmt.Sprintf("sdmp:idempotency:%d:%s", sourceID, webID)
}

func rawObjectKey(sourceID int, at time.Time, uid string) string {
	return port.JoinKey(
		"raw",
		strconv.Itoa(sourceID),
		fmt.Sprintf("%04d", at.Year()),
		fmt.Sprintf("%02d", at.Month()),
		uid+".json",
	)
}

func (s *ReceiveLeadService) lookupIdempotency(ctx context.Context, sourceID int, webID string) (int64, bool, error) {
	if s.cache == nil {
		return 0, false, fmt.Errorf("redis is not configured")
	}
	raw, ok, err := s.cache.HGet(ctx, idempotencyKey(sourceID, webID), idempotencyLeadField)
	if err != nil {
		return 0, false, err
	}
	if !ok || raw == "" {
		return 0, false, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false, nil
	}
	return id, true, nil
}

func (s *ReceiveLeadService) storeIdempotency(ctx context.Context, sourceID int, webID string, leadID int64) error {
	if s.cache == nil {
		return fmt.Errorf("redis is not configured")
	}
	key := idempotencyKey(sourceID, webID)
	if err := s.cache.HSet(ctx, key, map[string]string{
		idempotencyLeadField: strconv.FormatInt(leadID, 10),
	}); err != nil {
		return err
	}
	return s.cache.Expire(ctx, key, idempotencyTTL)
}
