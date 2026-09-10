package mcpserv

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"hostops/pfm/internal/inject"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (service *Service) chatBranch(
	ctx context.Context,
	request *mcp.CallToolRequest,
	input BranchInput,
) (*mcp.CallToolResult, ActionOutput, error) {
	if input.Name != "" && (strings.TrimSpace(input.Name) == "" || strings.ContainsAny(input.Name, "\r\n\x00")) {
		return nil, ActionOutput{}, fmt.Errorf("name must be one non-empty line when provided")
	}
	caller, err := service.backend.callerForRequest(ctx, requestMeta(request))
	if err != nil {
		return nil, ActionOutput{}, err
	}
	if refused, detail := service.selfCallerRefusal(caller); refused {
		return nil, ActionOutput{Status: "not_found", Code: inject.CodeUnknown, Message: detail}, nil
	}
	// Reaching here means the caller carried no threadId AND ambient identity is
	// allowed — the per-chat stdio server, launched by one chat and inheriting it
	// deliberately. The shared HTTP daemon never gets this far: selfCallerRefusal
	// above already refused it with noAmbientCallerRemedy. So this is NOT a blind
	// fork. The CLI runs as a descendant of a server that is a child of the chat,
	// reads CLAUDE_CODE_SESSION_ID from its own inherited environment, and derives
	// the parent's account and cache from that id. Refusing here would break
	// branch on precisely the transport that works.
	if !caller.valid {
		args := []string{"chat", "branch"}
		if input.Name != "" {
			args = append(args, "--name", input.Name)
		}
		return service.cliAction(ctx, args...)
	}
	if strings.TrimSpace(caller.row.Dir) == "" {
		return nil, ActionOutput{
			Status: "not_found", Code: inject.CodeUnknown,
			Message: fmt.Sprintf("MCP caller %q has no project directory to fork", caller.row.ID),
		}, nil
	}
	args := []string{
		"chat", "branch", "--engine", string(caller.row.Engine),
		"--session-id", caller.row.ID, "--cwd", caller.row.Dir,
	}
	if caller.row.Account != 0 {
		args = append(args, "--account", fmt.Sprint(caller.row.Account))
	}
	if input.Name != "" {
		args = append(args, "--name", input.Name)
	}
	return service.cliAction(ctx, args...)
}

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
