package telegram

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/firestore"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// const BotAPIKeyCtx = "BotAPIKeyCtx"
// const FireStoreClientKey = "FireStoreClientKey"

type BotAPIKeyType struct{}
type FireStoreKeyType struct{}

var (
	botApiKey    BotAPIKeyType
	firestoreKey FireStoreKeyType
)

func SetContextBotAPI(ctx context.Context, val *tgbotapi.BotAPI) context.Context {
	nctx := context.WithValue(ctx, botApiKey, val)
	return nctx
}

func GetContextBotAPI(ctx context.Context) (*tgbotapi.BotAPI, error) {
	botApi, ok := ctx.Value(botApiKey).(*tgbotapi.BotAPI)
	if !ok {
		return nil, fmt.Errorf("failed to get context bot api val")
	}
	return botApi, nil
}

func SetContextFireStoreClient(ctx context.Context, val *firestore.Client) context.Context {
	nctx := context.WithValue(ctx, firestoreKey, val)
	return nctx
}

func GetContextFireStoreClient(ctx context.Context) (*firestore.Client, error) {
	frClient, ok := ctx.Value(firestoreKey).(*firestore.Client)
	if !ok {
		return nil, fmt.Errorf("failed to get context key firestore client val")
	}
	return frClient, nil
}

type JTelegram struct {
	Bot    *tgbotapi.BotAPI
	chatID int64
	debug  bool
}

func SetupTelegram(botApiKey string, jOpts ...JTOpts) (*JTelegram, error) {
	if botApiKey == "" {
		return nil, fmt.Errorf("bot api empty")
	}

	jT := &JTelegram{}

	for _, jFunc := range jOpts {
		jFunc(jT)
	}

	bot, err := tgbotapi.NewBotAPI(botApiKey)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to init bot api: %w", err))
	}

	jT.Bot = bot

	return jT, nil
}

func (j *JTelegram) ListenUpdatesMsg(ctx context.Context, fr *firestore.Client) {
	getUpdates := tgbotapi.NewUpdate(0)
	getUpdates.Timeout = 60

	updatesChan := j.Bot.GetUpdatesChan(getUpdates)
	ctx = SetContextBotAPI(ctx, j.Bot)
	ctx = SetContextFireStoreClient(ctx, fr)

	for gu := range updatesChan {
		if gu.Message != nil {
			if gu.Message.IsCommand() {
				// TODO: add command handler
				j.telegramHandlerCommand(ctx, &gu)
			}
			// TODO: add command text
		}
	}
}

func (j *JTelegram) telegramHandlerCommand(ctx context.Context, update *tgbotapi.Update) {
	u := update
	switch u.Message.Command() {
	case "setup":
		req := SetupHandlerRequest{
			ChatID:   u.FromChat().ID,
			Username: u.FromChat().UserName,
		}
		HandlerSetup(ctx, req)
	default:

	}
}

func (j *JTelegram) SendChat(msg string) {
	j.chatID = int64(131109047)
	msgChat := tgbotapi.NewMessage(j.chatID, msg)
	j.Bot.Send(msgChat)
}

type JTOpts func(conf *JTelegram)

func WithDebug(b bool) JTOpts {
	return func(conf *JTelegram) {
		conf.debug = true
	}
}
