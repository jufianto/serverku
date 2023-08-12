package telegram

import (
	"context"
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramHandler interface {
	HandlerSetup()
}

type SetupHandlerRequest struct {
	ChatID   int64  `firestore:"chatID,omitempty"`
	Username string `firestore:"username,omitempty"`
}

func deferFunc(chatID int64, bapi *tgbotapi.BotAPI, err error) {
	if bapi == nil || err == nil {
		return
	}

	msg := fmt.Sprintf("got an error: %v", err)
	msgTele := tgbotapi.NewMessage(chatID, msg)
	bapi.Send(msgTele)
}

func HandlerSetup(ctx context.Context, req SetupHandlerRequest) {
	var err error
	var bapi *tgbotapi.BotAPI

	defer deferFunc(req.ChatID, bapi, err)

	bapi, err = GetContextBotAPI(ctx)
	if err != nil {
		log.Printf("error getting bot api: %v", err)
		return
	}

	frClient, err := GetContextFireStoreClient(ctx)
	if err != nil {
		log.Printf("error getting firestore client: %v", err)
		return
	}

	chatIDStr := fmt.Sprintf("%d", req.ChatID)
	_, err = frClient.Collection("serverku-user").Doc(chatIDStr).Set(ctx, req)
	if err != nil {
		log.Printf("failed to add data: %v", err)
		return
	}

	msg := fmt.Sprintf("success setup id %d and username %s", req.ChatID, req.Username)
	newMsg := tgbotapi.NewMessage(req.ChatID, msg)
	bapi.Send(newMsg)
	log.Println(msg)
}
