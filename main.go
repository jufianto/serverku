package main

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jufianto/serverku/pkg"
)

func main() {
	jbot, err := pkg.SetupTelegram("541876379:AAEeeMMHbwPP2ucTfHehqob8LzaGv7WZCiU", pkg.WithDebug(true))
	if err != nil {
		log.Printf("failed to setup telegam: %v", err)
	}
	fmt.Printf("jbot %+v \n", jbot)
	fmt.Println("bot name", jbot.Bot.Self.UserName)

	jbot.SendChat("hello from local")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := jbot.Bot.GetUpdatesChan(u)

	for up := range updates {
		if up.Message != nil {
			id := up.FromChat().ID
			log.Printf("%v - %v [%s] %s \n", id, up.Message.Chat.FirstName, up.Message.From.UserName, up.Message.Text)
			now := time.Now()
			msg := tgbotapi.NewMessage(id, fmt.Sprintf("the id from %s: %v", now.Format("02 Jan 06, 15:04"), id))
			res, _ := jbot.Bot.Send(msg)
			fmt.Println("RES", res)
		}
	}
}
