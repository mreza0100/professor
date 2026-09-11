package mcpserv

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"hostops/pfm/internal/inject"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (service *Service) chatGoal(
	ctx context.Context,
	request *mcp.CallToolRequest,
	input GoalInput,
) (*mcp.CallToolResult, InjectOutput, error) {
	goal := strings.TrimSpace(input.Goal)
	if goal == "" || strings.ContainsAny(goal, "\r\n\x00") {
		return nil, InjectOutput{}, fmt.Errorf("goal must be one non-empty line")
	}
	if utf8.RuneCountInString(goal) > 4000 {
		return nil, InjectOutput{}, fmt.Errorf("goal is %d characters; maximum is 4000", utf8.RuneCountInString(goal))
	}
	target := strings.TrimSpace(input.Target)
	if target == "" {
		target = "self"
	}
	injector, caller, err := service.injectorForRequest(ctx, request)
	if err != nil {
		return nil, InjectOutput{}, err
	}
	if refused, detail := service.selfCallerRefusal(caller); selfTarget(target) && refused {
		return nil, InjectOutput{Status: "not_found", Code: inject.CodeUnknown, Message: detail}, nil
	}
	result, err := injector.Inject(ctx, inject.Request{Target: target, Message: "/goal " + goal})
	return nil, outputFromInject(result), err
}
