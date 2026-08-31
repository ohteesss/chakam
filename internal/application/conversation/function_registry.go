package conversation

import (
	"chakam/internal/application/agreement"
	"chakam/internal/application/user"
	domainAgreement "chakam/internal/domain/agreement"
	"chakam/internal/infrastructure/whatsapp"
	"chakam/internal/pkg/validator"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

type FunctionRegistry struct {
	agreementService *agreement.Service
	userService      *user.Service
	whatsappClient   *whatsapp.Client
}

func NewFunctionRegistry(agreementService *agreement.Service, userService *user.Service, whatsappClient *whatsapp.Client) *FunctionRegistry {
	return &FunctionRegistry{agreementService: agreementService, userService: userService, whatsappClient: whatsappClient}
}

func (r *FunctionRegistry) GetAllDeclarations() []*genai.FunctionDeclaration {
	return []*genai.FunctionDeclaration{
		{
			Name:        "create_agreement",
			Description: "Creates a new agreement between two parties",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"agreement_type": {
						Type:        genai.TypeString,
						Enum:        []string{"SALE", "PAYMENT", "DELIVERY", "RECEIPT", "OTHER"},
						Description: "The type of agreement to create",
					},
					"amount": {
						Type:        genai.TypeNumber,
						Description: "The amount of the agreement",
					},
					"currency": {
						Type:        genai.TypeString,
						Description: "The currency of the agreement",
						Enum:        []string{"NGN", "USD", "EUR", "GBP", "CAD", "AUD", "NZD", "CHF", "CNY", "JPY", "KRW", "INR", "BDT", "BHD", "BRL", "BYN", "BYR", "CHF", "CNY", "CZK", "DKK", "EUR", "GBP", "HKD", "HUF", "IDR", "ILS", "INR", "JPY", "KRW", "KWD", "LKR", "LYD", "MAD", "MDL", "MXN", "MYR", "NOK", "NZD", "OMR", "PEN", "PHP", "PKR", "PLN", "QAR", "RON", "RUB", "SAR", "SEK", "SGD", "THB", "TRY", "TWD", "UAH", "USD", "VEF", "VND", "ZAR"},
					},
					"description": {
						Type:        genai.TypeString,
						Description: "The description of the agreement",
					},
					"counterparty_phone": {
						Type:        genai.TypeString,
						Description: "The phone number of the counterparty",
					},
				},
				Required: []string{"agreement_type", "description", "counterparty_phone"},
			},
		},
		{
			Name:        "confirm_agreement",
			Description: "Confirms a pending agreement between two parties",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"agreement_id": {
						Type:        genai.TypeString,
						Description: "The ID of the agreement to confirm",
					},
				},
				Required: []string{"agreement_id"},
			},
		},
		{
			Name:        "get_my_agreements",
			Description: "Gets my agreements and their status",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"filter": {
						Type:        genai.TypeString,
						Description: "The filter to apply to the agreements",
						Enum:        []string{"all", "pending", "confirmed", "created_by_me"},
					},
				},
				Required: []string{"filter"},
			},
		},
	}
}

func (r *FunctionRegistry) Execute(ctx context.Context, functionName string, args map[string]interface{}, userID string) (string, error) {
	switch functionName {
	case "create_agreement":
		return r.CreateAgreement(ctx, args, userID)
	case "confirm_agreement":
		return r.ConfirmAgreement(ctx, args, userID)
	case "get_my_agreements":
		return r.GetMyAgreements(ctx, args, userID)

	default:
		return "", fmt.Errorf("function %s not found", functionName)
	}
}

func (r *FunctionRegistry) CreateAgreement(ctx context.Context, args map[string]interface{}, userID string) (string, error) {

	agreementType, _ := args["agreement_type"].(string)
	description, _ := args["description"].(string)
	counterpartyPhone, _ := args["counterparty_phone"].(string)
	amount, _ := args["amount"].(float64) // 0 if the model didn't provide one

	currency, _ := args["currency"].(string)
	if currency == "" {
		currency = "NGN"
	}

	amountMinor, err := domainAgreement.NewMoney(amount, currency)
	if err != nil {
		return "", err
	}

	from, err := r.userService.GetUserById(ctx, userID)
	if err != nil {
		return "", err
	}

	counterpartyE164, err := validator.ToE164(counterpartyPhone, validator.RegionFor(from.PhoneNumber))
	if err != nil {
		return fmt.Sprintf("That phone number doesn't look right: %q. Please send it with the country code, like +2348031234567.", counterpartyPhone), nil
	}

	u, err := r.userService.CreateUser(ctx, counterpartyE164, nil)
	if err != nil {
		return "", err
	}

	// The model marks where each party is named with {initiator}/{counterparty}
	// rather than writing the digits itself: it is never told the initiator's
	// number, and a mistyped digit in an immutable record would bind the wrong
	// person. Substituting here guarantees both numbers are the canonical E.164.
	description = withPartyPhones(description, from.PhoneNumber, counterpartyE164)

	newAgreement := &domainAgreement.Agreement{
		AgreementType: agreementType,
		Amount: domainAgreement.Money{
			AmountMinor: amountMinor.AmountMinor,
			Currency:    amountMinor.Currency,
		},
		Description:    description,
		CounterpartyID: u.PublicID,
		InitiatorID:    userID,
		Status:         "PENDING",
	}
	// if from.PhoneNumber == counterpartyPhone {
	// 	return "", fmt.Errorf("you cannot create an agreement with yourself")
	// }
	a, err := r.agreementService.CreateAgreement(ctx, newAgreement)
	if err != nil {
		return "", err
	}
	if _, err := r.whatsappClient.SendTemplateConfirmAgreement(ctx, counterpartyE164, from.PhoneNumber, description, a.ID); err != nil {
		return "", err
	}
	return "Agreement created successfully", nil
}

func (r *FunctionRegistry) ConfirmAgreement(ctx context.Context, args map[string]interface{}, userID string) (string, error) {
	agreementID, _ := args["agreement_id"].(string)
	if agreementID != "" {
		return r.confirm(ctx, agreementID, userID)
	}
	pendingAgreements, err := r.agreementService.GetPendingForCounterparty(ctx, userID)
	if err != nil {
		return "", err
	}
	switch len(pendingAgreements) {
	case 0:
		return "You have no pending agreements waiting for your confirmation.", nil
	case 1:
		return r.confirm(ctx, pendingAgreements[0].ID, userID)
	default:
		return "You have multiple pending agreements. Please specify the agreement ID.", nil
	}
}

func (r *FunctionRegistry) confirm(ctx context.Context, id, userID string) (string, error) {
	a, err := r.agreementService.GetAgreementById(ctx, id)
	if err != nil {
		return "", err
	}
	if a.CounterpartyID != userID {
		return "Only the person who received this agreement can confirm it.", nil
	}
	if a.Status != "PENDING" {
		return fmt.Sprintf("This agreement is already %s.", strings.ToLower(a.Status)), nil
	}
	_, err = r.agreementService.ConfirmAgreement(ctx, id)
	if err != nil {
		return "", err
	}
	return "Agreement confirmed successfully", nil
}

func (r *FunctionRegistry) GetMyAgreements(ctx context.Context, args map[string]interface{}, userID string) (string, error) {
	filter, _ := args["filter"].(string)
	var response []map[string]interface{}

	agreements, err := r.agreementService.GetAgreements(ctx, filter, userID)
	if err != nil {
		return "", err
	}
	for _, a := range agreements {
		counterparty, err := r.userService.GetUserById(ctx, a.CounterpartyID)
		if err != nil {
			return "", err
		}
		name := ""
		if counterparty.Name != nil {
			name = *counterparty.Name
		}
		response = append(response, map[string]interface{}{
			"id":             a.ID,
			"agreement_type": a.AgreementType,
			"description":    a.Description,
			"amount":         a.Amount,
			"currency":       a.Amount.Currency,
			"counterparty": map[string]interface{}{
				"phone": counterparty.PhoneNumber,
				"name":  name,
			},
			"status":     a.Status,
			"created_at": a.CreatedAt,
			"updated_at": a.UpdatedAt,
		})
	}
	out, err := json.Marshal(response)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Party placeholders the model is instructed to leave in the description.
const (
	initiatorToken    = "{initiator}"
	counterpartyToken = "{counterparty}"
)

func withPartyPhones(description, initiatorPhone, counterpartyPhone string) string {
	description = strings.TrimSpace(description)

	hasInitiator := strings.Contains(description, initiatorToken)
	hasCounterparty := strings.Contains(description, counterpartyToken)

	description = strings.ReplaceAll(description, initiatorToken, initiatorPhone)
	description = strings.ReplaceAll(description, counterpartyToken, counterpartyPhone)

	var missing []string
	if !hasInitiator {
		missing = append(missing, "initiator "+initiatorPhone)
	}
	if !hasCounterparty {
		missing = append(missing, "counterparty "+counterpartyPhone)
	}
	if len(missing) == 0 {
		return description
	}

	if description != "" && !strings.HasSuffix(description, ".") {
		description += "."
	}
	return strings.TrimSpace(description + " (Parties: " + strings.Join(missing, "; ") + ".)")
}


func stripPartyTokens(text string) string {
	text = strings.ReplaceAll(text, initiatorToken, "you")
	text = strings.ReplaceAll(text, counterpartyToken, "the other party")
	return text
}
