package alerting

import (
	"context"
	"fmt"
	"lens-bot/lens-bot-1/config"
	"lens-bot/lens-bot-1/sqldata"
	"log"

	"github.com/shomali11/slacker"
)

func printCommandEvents(analyticsChannel <-chan *slacker.CommandEvent) {
	for event := range analyticsChannel {
		fmt.Println("Command Events")
		fmt.Println(event.Timestamp)
		fmt.Println(event.Command)
		fmt.Println(event.Parameters)
		fmt.Println(event.Event)
		fmt.Println()
	}
}

// Send allows bot to send a telegram alert to the configured chatID
func RegisterSlack() {
	// Create a new client to slack by giving token
	// Set debug to true while developing
	config, err1 := config.ReadConfigFromFile()
	//err := bot.Listen(ctx)
	if err1 != nil {
		log.Fatal(err1)
	}
	bot := slacker.NewClient(config.Slack.BotToken, config.Slack.AppToken)

	go printCommandEvents(bot.CommandEvents())

	bot.Command("register <chain_id> <validator_address>", &slacker.CommandDefinition{
		Description: "register",
		Handler: func(botCtx slacker.BotContext, request slacker.Request, response slacker.ResponseWriter) {
			chain_id := request.Param("chain_id")
			validator_address := request.Param("validator_address")
			sqldata.ChainDataInsert(chain_id, validator_address)
			r := fmt.Sprintf("your respose has been recorded %s", validator_address)
			response.Reply(r)
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := bot.Listen(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
