package alerting

import (
	"context"
	"fmt"
	"log"

	"github.com/likhita-809/lens-bot/config"
	"github.com/likhita-809/lens-bot/database"
	"github.com/likhita-809/lens-bot/keyshandler"
	"github.com/likhita-809/lens-bot/voting"

	"github.com/shomali11/slacker"
	"github.com/slack-go/slack"
)

type Slackbot struct {
	bot  *slacker.Slacker
	db   *database.Sqlitedb
	cfg  *config.Config
	key  *keyshandler.Keys
	vote *voting.Vote
}

// Creates a new bot client
func NewBotClient(config *config.Config, db *database.Sqlitedb) *Slackbot {
	bot := slacker.NewClient(config.Slack.BotToken, config.Slack.AppToken)
	return &Slackbot{
		bot: bot,
		db:  db,
		cfg: config,
	}
}

// Creates and initialises commands
func (a *Slackbot) Initializecommands() error {

	// Command to register validator address with chain name
	a.bot.Command("register-validator <chainName> <validatorAddress>", &slacker.CommandDefinition{
		Description: "register a new validator",
		Examples:    []string{"/register-validator cosmoshub cosmos1a..."},
		Handler: func(botCtx slacker.BotContext, request slacker.Request, response slacker.ResponseWriter) {
			chainName := request.Param("chainName")
			validatorAddress := request.Param("validatorAddress")
			isExists := a.db.HasValidator(validatorAddress)
			if isExists {
				response.Reply("Validator is already registered")
			} else {
				a.db.AddValidator(chainName, validatorAddress)
				r := fmt.Sprintf("Your validator %s is successfully registered", validatorAddress)
				response.Reply(r)
			}
		},
	})
	//Creates keys which need to be funded and will vote in the place of validators
	a.bot.Command("create-key <chainName> <keyName_Optional>", &slacker.CommandDefinition{
		Description: "create a new account with key name",
		Examples:    []string{"create-key my_key"},
		Handler: func(botCtx slacker.BotContext, request slacker.Request, response slacker.ResponseWriter) {
			keyName := request.StringParam("keyName_Optional", "default")
			chainName := request.Param("chainName")
			err := a.key.CreateKeys(chainName, keyName)
			if err != nil {
				response.Reply(err.Error())
			} else {
				NewSlackAlerter().Send(fmt.Sprintf("Successfully created your key with name %s", keyName), a.cfg.Slack.BotToken, a.cfg.Slack.ChannelID)
			}
		},
	})
	//votes on proposals based on proposalid,validator address,vote and key
	a.bot.Command(
		"vote <chain_id> <proposal_id> <validator_address> <vote_option> <from_key> <metadata_optional> <memo_optional> <gas_units_optional> <fees_optional>",
		&slacker.CommandDefinition{
			Description: "vote",
			Examples:    []string{"/vote cosmoshub 123 YES memodata 300000 0.25uatom "},
			Handler: func(botCtx slacker.BotContext, request slacker.Request, response slacker.ResponseWriter) {
				chainID := request.Param("chain_id")
				pID := request.Param("proposal_id")
				valAddr := request.Param("validator_address")
				voteOption := request.Param("vote_option")
				fromKey := request.Param("from_key")
				metadata := request.StringParam("metadata_optional", "")
				memo := request.StringParam("memo_optional", "")
				gas := request.StringParam("gas_units_optional", "")
				fees := request.StringParam("fees_optional", "")
				err := a.vote.ExecVote(chainID, pID, valAddr, voteOption, fromKey, metadata, memo, gas, fees)
				if err != nil {
					log.Printf("error on executing vote: %v", err)
				}
				a := fmt.Sprintf("%v", err.Error())
				response.Reply(a)
			},
		},
	)
	//Lists all the keys stored in the database
	a.bot.Command("list-keys", &slacker.CommandDefinition{
		Description: "lists all keys",
		Examples:    []string{"list-all"},
		Handler: func(botCtx slacker.BotContext, request slacker.Request, response slacker.ResponseWriter) {
			r, err := a.db.GetKeys()
			if err != nil {
				response.ReportError(err)
			} else {

				apiClient := botCtx.APIClient()
				event := botCtx.Event()

				var blocks []slack.Block
				for _, val := range r {
					blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*%s* ---- *%s*", val.ChainName, val.KeyName), false, false),
						nil, nil))
				}

				attachment := []slack.Block{
					slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "Network ---- Key name", false, false)),
				}
				attachment = append(attachment, blocks...)

				if event.ChannelID != "" {
					_, _, err := apiClient.PostMessage(event.ChannelID, slack.MsgOptionBlocks(attachment...))
					if err != nil {
						response.ReportError(err)
					}
				}
			}
		},
	})

	//Command to list all registered validators
	a.bot.Command("list-validators", &slacker.CommandDefinition{
		Description: "lists all chains with associated validator addresses",
		Examples:    []string{"list-validators"},
		Handler: func(botCtx slacker.BotContext, request slacker.Request, response slacker.ResponseWriter) {
			validators, err := a.db.GetValidators()
			if err != nil {
				response.ReportError(err)
			}

			apiClient := botCtx.APIClient()
			event := botCtx.Event()

			var blocks []slack.Block
			for _, val := range validators {
				blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*%s* ---- *%s*", val.ChainName, val.Address), false, false),
					nil, nil))
			}

			attachment := []slack.Block{
				slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "Network ---- Validator address", false, false)),
			}
			attachment = append(attachment, blocks...)
			if event.ChannelID != "" {
				_, _, err := apiClient.PostMessage(event.ChannelID, slack.MsgOptionBlocks(attachment...))
				if err != nil {
					response.ReportError(err)
				}
			}
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := a.bot.Listen(ctx)
	if err != nil {
		return fmt.Errorf("%s", err)
	}
	return err
}

// Send allows bot to send a slack alert to the configured channelID
func (s slackAlert) Send(msgText, botToken string, channelID string) error {
	// Create a new client to slack by giving token
	// Set debug to true while developing
	client := slack.New(botToken, slack.OptionDebug(true))

	// Create the Slack attachment that we will send to the channel
	attachment := slack.Attachment{
		Pretext: "Lens Bot Message",
		Title:   msgText,
	}

	// PostMessage will send the message away.
	// First parameter is just the channelID, makes no sense to accept it
	_, timestamp, err := client.PostMessage(
		channelID,
		slack.MsgOptionAttachments(attachment),
	)
	if err != nil {
		return err
	}
	log.Printf("Message sent at %s", timestamp)
	return nil
}
