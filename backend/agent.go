package main

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const instruction = `Eres krat, un asistente de voz cálido y curioso que conversa en tiempo real con el usuario.
En cada turno recibes el audio de lo que el usuario acaba de decir y, a veces, una imagen de lo que ve su cámara en ese momento.
Escucha el audio y responde a lo que dice. Nunca lo transcribas ni lo repitas.
Usa la imagen solo cuando sea relevante, por ejemplo si te muestra algo o te pregunta qué ves.
Tú no tienes cuerpo ni manos: lo que aparece en la imagen lo tiene el usuario. Háblale de tú ("tienes", "sostienes", "me enseñas").
Responde en el idioma del usuario (por defecto español), de forma natural y breve: de una a tres frases cortas.
No uses listas, markdown ni emojis, porque tu respuesta se convertirá en voz.`

// The model's own ASR trigger; Spanish "transcribe this" prompts make it answer the question instead.
const transcribePrompt = "Perform ASR."

// Agent wraps the Eino ADK agent. Tools/MCP/RAG get registered here via ToolsConfig.
type Agent struct {
	model  model.ToolCallingChatModel
	runner *adk.Runner
}

func newAgent(ctx context.Context, cfg config) (*Agent, error) {
	cm, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{BaseURL: cfg.OllamaURL, Model: cfg.Model})
	if err != nil {
		return nil, err
	}
	a, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "krat",
		Description: "Asistente multimodal de voz y visión",
		Instruction: instruction,
		Model:       cm,
	})
	if err != nil {
		return nil, err
	}
	return &Agent{model: cm, runner: adk.NewRunner(ctx, adk.RunnerConfig{Agent: a, EnableStreaming: true})}, nil
}

// media builds a multimodal user message. The eino ollama adapter only accepts image parts,
// but Ollama sniffs the bytes, so the WAV travels as an "image" part and is decoded as audio.
func media(text, audioB64, imageB64 string) *schema.Message {
	var parts []schema.MessageInputPart
	if text != "" {
		parts = append(parts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: text})
	}
	for _, p := range []struct{ data, mime string }{{imageB64, "image/jpeg"}, {audioB64, "audio/wav"}} {
		if p.data == "" {
			continue
		}
		d := p.data
		parts = append(parts, schema.MessageInputPart{
			Type:  schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &d, MIMEType: p.mime}},
		})
	}
	return &schema.Message{Role: schema.User, UserInputMultiContent: parts}
}

// Reply streams the agent's answer to the current turn, calling onDelta per chunk, and returns the full text.
func (a *Agent) Reply(ctx context.Context, history []*schema.Message, audioB64, imageB64 string, onDelta func(string)) (string, error) {
	text := "Responde a lo que te digo en el audio."
	if imageB64 != "" {
		text = "Responde a lo que te digo en el audio. La imagen es mi cámara ahora mismo: lo que aparece es lo que yo tengo o te enseño."
	}
	msgs := append(append([]*schema.Message{}, history...), media(text, audioB64, imageB64))

	var full strings.Builder
	iter := a.runner.Run(ctx, msgs)
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			return full.String(), ev.Err
		}
		if ev.Output == nil || ev.Output.MessageOutput == nil || ev.Output.MessageOutput.Role != schema.Assistant {
			continue
		}
		mv := ev.Output.MessageOutput
		if !mv.IsStreaming {
			full.WriteString(mv.Message.Content)
			onDelta(mv.Message.Content)
			continue
		}
		for {
			chunk, err := mv.MessageStream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				mv.MessageStream.Close()
				return full.String(), err
			}
			if chunk.Content != "" {
				full.WriteString(chunk.Content)
				onDelta(chunk.Content)
			}
		}
		mv.MessageStream.Close()
	}
	return full.String(), ctx.Err()
}

// Transcribe returns what the user said, for captions and the text-only history.
func (a *Agent) Transcribe(ctx context.Context, audioB64 string) (string, error) {
	resp, err := a.model.Generate(ctx, []*schema.Message{schema.SystemMessage(transcribePrompt), media("", audioB64, "")})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}
