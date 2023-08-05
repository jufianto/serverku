package pkg

import (
	"errors"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type JTelegram struct {
	Bot    *tgbotapi.BotAPI
	chatID int64
	debug  bool
}

func SetupTelegram(botApi string, jOpts ...JTOpts) (*JTelegram, error) {
	if botApi == "" {
		return nil, fmt.Errorf("bot api empty")
	}

	jT := &JTelegram{}

	for _, jFunc := range jOpts {
		jFunc(jT)
	}

	bot, err := tgbotapi.NewBotAPI(botApi)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to init bot api: %w", err))
	}

	jT.Bot = bot

	return jT, nil
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
