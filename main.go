package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jufianto/serverku/pkg"
	pkg_tele "github.com/jufianto/serverku/pkg/telegram"
	"github.com/spf13/viper"
	"google.golang.org/api/iterator"
)

var config *viper.Viper

func init() {
	config = viper.New()
	config.SetConfigFile("config")
	config.SetConfigType("ini")
	if err := config.ReadInConfig(); err != nil {
		log.Fatalf("failed to read viper config: %s", err)
	}
}

func main() {
	apiKey := config.GetString("telegram.apiKey")
	jbot, err := pkg_tele.SetupTelegram(apiKey, pkg_tele.WithDebug(true))
	if err != nil {
		log.Fatalf("failed to setup telegram: %v", err)
	}

	projectID := config.GetString("gcp.projectID")
	if projectID == "" {
		log.Fatalf("project id must not empty")
	}

	ctx := context.Background()

	frClient := pkg.SetupFirestoreClient(ctx, projectID)
	if frClient == nil {
		log.Fatalf("failed to init firestore")
	}

	if false {
		prepareRetrieve(ctx, frClient)
	}

	jbot.ListenUpdatesMsg(ctx, frClient)
}

func exSendTelegram() {
	apiKey := config.GetString("telegram.apiKey")
	jbot, err := pkg_tele.SetupTelegram(apiKey, pkg_tele.WithDebug(true))
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

func exReadFirestore() {
	ctx := context.Background()
	client := pkg.SetupFirestoreClient(ctx, "")

	iter := client.Collection("serverku-notify").Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("failed to iterate: %v", err)
		}
		fmt.Println(doc.Data())
	}
}

type City struct {
	Name       string
	State      string
	Country    string
	Capital    bool
	Population int
}

func prepareRetrieve(ctx context.Context, client *firestore.Client) error {
	cities := []struct {
		id string
		c  City
	}{
		{id: "SF", c: City{Name: "San Francisco", State: "CA", Country: "USA", Capital: false, Population: 860000}},
		{id: "LA", c: City{Name: "Los Angeles", State: "CA", Country: "USA", Capital: false, Population: 3900000}},
		{id: "DC", c: City{Name: "Washington D.C.", Country: "USA", Capital: true, Population: 680000}},
		{id: "TOK", c: City{Name: "Tokyo", Country: "Japan", Capital: true, Population: 9000000}},
		{id: "BJ", c: City{Name: "Beijing", Country: "China", Capital: true, Population: 21500000}},
	}
	for _, c := range cities {
		_, err := client.Collection("cities_example").Doc(c.id).Set(ctx, c.c)
		if err != nil {
			return err
		}
	}
	log.Println("success save")
	return nil
}
