package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"

	"chakam/internal/application/agreement"
	"chakam/internal/application/conversation"
	"chakam/internal/application/user"
	domainAgreement "chakam/internal/domain/agreement"
	"chakam/internal/infrastructure/whatsapp"
	"chakam/internal/pkg/logger"
	"chakam/internal/pkg/validator"

	"go.uber.org/zap"
)

type MessageHandler struct {
	log              *zap.Logger
	userService      *user.Service
	agreementService *agreement.Service
	whatsappClient   *whatsapp.Client
	llmOrchestrator  *conversation.LLMOrchestrator
}

func NewMessageHandler(log *zap.Logger, userService *user.Service, agreementService *agreement.Service, whatsappClient *whatsapp.Client, llmOrchestrator *conversation.LLMOrchestrator) *MessageHandler {
	return &MessageHandler{log: log, userService: userService, agreementService: agreementService, whatsappClient: whatsappClient, llmOrchestrator: llmOrchestrator}
}

func (h *MessageHandler) HandleProcessMessage(ctx context.Context, t *asynq.Task) error {
	payload := ProcessMessagePayload{}
	err := json.Unmarshal(t.Payload(), &payload)
	if err != nil {
		return err
	}
	h.log.Info("processing message", zap.String("message_id", valueOrEmpty(payload.MessageID)), zap.String("type", payload.Type))

	// Meta sends wa_id as bare digits with no "+", so restore it before the
	// number is used as an identity. A malformed wa_id will never become valid,
	// so skip retries rather than burning the queue's retry budget on it.
	from, err := validator.FromWaID(payload.From)
	if err != nil {
		h.log.Error("invalid wa_id; dropping message", zap.String("wa_id", logger.MaskPhone(payload.From)), zap.Error(err))
		return fmt.Errorf("invalid wa_id %q: %w", logger.MaskPhone(payload.From), asynq.SkipRetry)
	}

	u, err := h.userService.CreateUser(ctx, from, payload.Name)
	if err != nil {
		h.log.Error("failed to ensure user exists", zap.String("wa_id", logger.MaskPhone(from)), zap.Error(err))
		return err
	}
	h.log.Info("user ensured", zap.String("user_id", u.PublicID))

	if payload.MessageID == nil {
		return nil
	}

	if payload.Type == "text" {
		response, err4 := h.llmOrchestrator.HandleMessage(ctx, conversation.ProcessMessagePayload{
			From:      from,
			Name:      payload.Name,
			Type:      payload.Type,
			MessageID: payload.MessageID,
			Body:      payload.Body,
		})
		if err4 != nil {
			h.log.Error("failed to generate response", zap.String("to", logger.MaskPhone(from)), zap.Error(err4))
			return err4
		}
		_, err5 := h.whatsappClient.SendText(ctx, from, response.Message)
		if err5 != nil {
			h.log.Error("failed to send text message", zap.String("to", logger.MaskPhone(from)), zap.Error(err5))
			return err5
		}

		h.log.Info("text message sent", zap.String("to", logger.MaskPhone(from)))
	} else if payload.Type == "button" {
		h.log.Info("processing button message", zap.String("message_id", valueOrEmpty(payload.MessageID)))

		if payload.Button == nil {
			h.log.Error("button message missing button payload; dropping", zap.String("message_id", valueOrEmpty(payload.MessageID)))
			return nil
		}

		action, agreementID, ok := strings.Cut(payload.Button.Payload, ":")
		if !ok {
			h.log.Error("invalid button payload", zap.String("message_id", valueOrEmpty(payload.MessageID)))
			return fmt.Errorf("failed to cut button payload")
		}

		if action == "ACCEPT" {
			h.log.Info("accepting agreement", zap.String("agreement_id", agreementID))
			agr, err := h.agreementService.ConfirmAgreement(ctx, agreementID)
			if err != nil {
				h.log.Error("failed to confirm agreement", zap.String("agreement_id", agreementID), zap.Error(err))
				return err
			}
			initiator, err := h.userService.GetUserById(ctx, agr.InitiatorID)
			if err != nil {
				h.log.Error("failed to get initiator", zap.String("user_id", agr.InitiatorID), zap.Error(err))
				return err
			}
			response := formatConfirmation(agr, initiator.PhoneNumber, from)
			for _, num := range []string{from, initiator.PhoneNumber} {
				_, err = h.whatsappClient.SendText(ctx, num, response)
				if err != nil {
					h.log.Error("failed to send text message", zap.String("to", logger.MaskPhone(num)), zap.Error(err))
					return err
				}
			}
			h.log.Info("text message sent", zap.String("to", logger.MaskPhone(from)))
			return nil
		} else if action == "REJECT" {
			h.log.Info("rejecting agreement", zap.String("agreement_id", agreementID))
			agr, err := h.agreementService.RejectAgreement(ctx, agreementID)
			if err != nil {
				h.log.Error("failed to reject agreement", zap.String("agreement_id", agreementID), zap.Error(err))
				return err
			}
			initiator, err := h.userService.GetUserById(ctx, agr.InitiatorID)
			if err != nil {
				h.log.Error("failed to get initiator", zap.String("user_id", agr.InitiatorID), zap.Error(err))
				return err
			}
			response := formatRejection(agr, initiator.PhoneNumber, from)
			_, err = h.whatsappClient.SendText(ctx, from, response)
			if err != nil {
				h.log.Error("failed to send text message", zap.String("to", logger.MaskPhone(from)), zap.Error(err))
				return err
			}
			h.log.Info("text message sent", zap.String("to", logger.MaskPhone(from)))
			return nil
		}

	} else {
		h.log.Error("unsupported message type", zap.String("type", payload.Type))

		_, err = h.whatsappClient.SendText(ctx, from, "We don't support this message type yet, send text messages only")
		if err != nil {
			h.log.Error("failed to send text message", zap.String("to", logger.MaskPhone(from)), zap.Error(err))
			return err
		}
		h.log.Info("text message sent", zap.String("to", logger.MaskPhone(from)))
		return nil
	}

	return nil
}

func (h *MessageHandler) HandleSendMessage(ctx context.Context, t *asynq.Task) error {
	payload := SendMessagePayload{}
	err := json.Unmarshal(t.Payload(), &payload)
	if err != nil {
		return err
	}

	h.log.Info("sending queued message", zap.String("to", logger.MaskPhone(payload.To)))
	_, err = h.whatsappClient.SendText(ctx, payload.To, payload.Message)
	if err != nil {
		h.log.Error("failed to send text message", zap.String("to", logger.MaskPhone(payload.To)), zap.Error(err))
		return err
	}
	h.log.Info("queued message sent", zap.String("to", logger.MaskPhone(payload.To)))
	return nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func formatConfirmation(a *domainAgreement.Agreement, initiatorPhone string, counterpartyPhone string) string {
	amount, _ := a.Amount.Major()
	return fmt.Sprintf(
		"✅ Agreement confirmed & recorded.\n\n"+
			"Agreement Created by: %s\nWith: %s\nAmount: %s %.2f\nDetails: %s\n\n"+
			"Confirmed: %s\nRef: %s\n\nThis record cannot be changed.",
		initiatorPhone, counterpartyPhone,
		a.Amount.Currency, amount, a.Description,
		a.UpdatedAt.Format("2 Jan 2006, 3:04 PM"), a.ID,
	)
}

func formatRejection(a *domainAgreement.Agreement, initiatorPhone string, counterpartyPhone string) string {
	amount, _ := a.Amount.Major()
	return fmt.Sprintf(
		"❌ Agreement rejected.\n\n"+
			"Agreement Created by: %s\nWith: %s\nAmount: %s %.2f\nDetails: %s\n\n"+
			"Rejected: %s\nRef: %s\n\nThis record cannot be changed.",
		initiatorPhone, counterpartyPhone,
		a.Amount.Currency, amount, a.Description,
		a.UpdatedAt.Format("2 Jan 2006, 3:04 PM"), a.ID,
	)
}
