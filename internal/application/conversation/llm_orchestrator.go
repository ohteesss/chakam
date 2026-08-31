package conversation

import (
	"chakam/internal/application/session"
	"chakam/internal/application/user"
	domainSession "chakam/internal/domain/session"
	"chakam/internal/infrastructure/gemini"
	"chakam/internal/pkg/logger"
	"chakam/internal/pkg/validator"
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/genai"
)

type ProcessMessagePayload struct {
	From      string
	Name      *string
	Type      string
	MessageID *string
	Body      string
}

type LLMOrchestrator struct {
	geminiClient     *gemini.Client
	sessionService   *session.Service
	userService      *user.Service
	functionRegistry *FunctionRegistry
}

type OrchestratorResponse struct {
	Message string
}

func NewLLMOrchestrator(geminiClient *gemini.Client, sessionService *session.Service, userService *user.Service, functionRegistry *FunctionRegistry) *LLMOrchestrator {
	return &LLMOrchestrator{geminiClient: geminiClient, sessionService: sessionService, userService: userService, functionRegistry: functionRegistry}
}

func (o *LLMOrchestrator) HandleMessage(ctx context.Context, messagePayload ProcessMessagePayload) (*OrchestratorResponse, error) {
	log := logger.New()
	// The queue handler normalises before enqueuing, but HandleMessage is also
	// reachable directly, so convert here too. FromWaID is idempotent for a
	// value that is already E.164.
	from, err := validator.FromWaID(messagePayload.From)
	if err != nil {
		return nil, fmt.Errorf("handle message: %w", err)
	}
	u, err := o.userService.CreateUser(ctx, from, messagePayload.Name)
	if err != nil {
		return nil, err
	}
	if u.IsBlocked {
		return &OrchestratorResponse{
			Message: "Your account is currently suspended. Please contact support to reactivate your account.",
		}, nil
	}
	err = o.sessionService.AddMessageToSession(ctx, u.PublicID, domainSession.Message{Role: "user", Content: messagePayload.Body})

	if err != nil {
		return nil, err
	}

	contents, err := o.BuildLLMContext(ctx, u.PublicID)
	if err != nil {
		return nil, err
	}

	functionDeclarations := o.functionRegistry.GetAllDeclarations()

	response, err := o.geminiClient.Generate(ctx, gemini.GenerateContentRequest{
		Contents:             contents,
		SystemInstruction:    buildSystemPrompt(),
		FunctionDeclarations: functionDeclarations,
	})
	if err != nil {
		return nil, err
	}

	log.Info("Gemini response received",
		zap.Int("function_calls", len(response.FunctionCalls())),
		zap.Int("response_length", len(response.Text())),
	)

	if len(response.FunctionCalls()) > 0 {
		contents = append(contents, response.Candidates[0].Content)
		for _, functionCall := range response.FunctionCalls() {
			result, execErr := o.functionRegistry.Execute(ctx, functionCall.Name, functionCall.Args, u.PublicID)
			if execErr != nil {
				log.Error("function execution failed", zap.String("fn", functionCall.Name), zap.Error(execErr))
				result = "error: " + execErr.Error()
			}
			contents = append(contents, genai.NewContentFromParts(
				[]*genai.Part{genai.NewPartFromFunctionResponse(functionCall.Name, map[string]any{"result": result})},
				genai.RoleUser,
			))

		}
		log.Info("function responses sent to Gemini", zap.Int("context_items", len(contents)))
		response, err = o.geminiClient.Generate(ctx, gemini.GenerateContentRequest{
			Contents:             contents,
			SystemInstruction:    buildSystemPrompt(),
			FunctionDeclarations: functionDeclarations,
		})
		if err != nil {
			return nil, err
		}
	}

	// The party placeholders are an internal protocol between the system prompt
	// and CreateAgreement. If the model echoes one into its reply — most often
	// when reading a draft description back for confirmation — the user would
	// see raw braces. Strip them before the text is stored or sent, so the
	// leaked token cannot reach the user or be copied back out of history.
	message := stripPartyTokens(response.Text())

	if message != "" {
		err = o.sessionService.AddMessageToSession(ctx, u.PublicID, domainSession.Message{Role: "assistant", Content: message})
		if err != nil {
			return nil, err
		}
	}

	return &OrchestratorResponse{
		Message: message,
	}, nil

}

func (o *LLMOrchestrator) BuildLLMContext(ctx context.Context, userID string) ([]*genai.Content, error) {
	history, err := o.sessionService.GetConversationHistory(ctx, userID)
	if err != nil {
		return nil, err
	}
	contents := make([]*genai.Content, 0, len(history))
	for _, message := range history {
		role := genai.RoleUser
		if message.Role == "assistant" {
			role = genai.RoleModel
		}
		contents = append(contents, genai.NewContentFromText(message.Content, genai.Role(role)))
	}
	return contents, nil
}
