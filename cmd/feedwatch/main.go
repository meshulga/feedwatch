package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"feedwatch/internal/bot"
	"feedwatch/internal/client"
	"feedwatch/internal/config"
	"feedwatch/internal/pipeline"
)

func main() {
	appIDStr := os.Getenv("APP_ID")
	appHash := os.Getenv("APP_HASH")
	if appIDStr == "" || appHash == "" {
		log.Fatal("APP_ID and APP_HASH environment variables must be set\n" +
			"Get them at https://my.telegram.org/apps")
	}
	appID, err := strconv.Atoi(appIDStr)
	if err != nil {
		log.Fatalf("invalid APP_ID %q: %v", appIDStr, err)
	}

	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	c := client.New(appID, appHash, "session.json", cfg.OwnerID())
	handler := bot.NewHandler(cfg, c)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err = c.Run(ctx, func(ctx context.Context, event client.MessageEvent) {
		if event.IsOwnerCommand {
			reply := handler.Handle(ctx, event.Text)
			if reply == "" {
				return
			}
			if sendErr := c.SendMessage(ctx, event.ChatID, event.ChatAccessHash, reply); sendErr != nil {
				log.Printf("send reply: %v", sendErr)
			}
			return
		}

		if event.Text == "" {
			return
		}

		matched := pipeline.NewRouter(cfg.Pipelines()).Route(event.ChatID, event.Text)
		for _, p := range matched {
			if p.Output == nil {
				continue
			}
			fromPeer := pipeline.Peer{
				ID:         event.ChatID,
				AccessHash: event.ChatAccessHash,
				Type:       event.ChatType,
			}
			if fwdErr := c.ForwardMessage(ctx, fromPeer, event.MessageID, *p.Output); fwdErr != nil {
				log.Printf("forward pipeline %q to %d: %v", p.ID, p.Output.ID, fwdErr)
			}
		}
	})

	if err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
