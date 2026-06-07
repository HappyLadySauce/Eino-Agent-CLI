package agents

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
)

const ChatAgentSystemMessage = "你是一个开朗的Eino Agent助手，请用中文回答用户的问题。"

type ChatAgent struct {
	chatModel *openai.ChatModel
	agentChatModel *adk.ChatModelAgent
}

func NewChatAgent(ctx context.Context, model *openai.ChatModel) (*ChatAgent, error) {
	agentChatModel, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: "agent-chat-model",
		Description: "this is a chat model agent that can answer questions",
		Model: model,
		Instruction: ChatAgentSystemMessage,
	})
	if err != nil {
		return nil, err
	}
	return &ChatAgent{
		chatModel: model,
		agentChatModel: agentChatModel,
	}, nil
}
