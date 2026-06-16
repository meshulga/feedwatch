package client

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"

	"feedwatch/internal/pipeline"
)

// MessageEvent is delivered for each relevant incoming message.
type MessageEvent struct {
	ChatID         int64
	ChatAccessHash int64
	ChatType       string // "channel", "chat", or "user"
	SenderID       int64
	MessageID      int
	Text           string
	// IsOwnerCommand is true when the sender is the configured owner,
	// including messages in Saved Messages (outgoing-to-self).
	IsOwnerCommand bool
}

// Handler is called for each message event.
type Handler func(ctx context.Context, event MessageEvent)

type Client struct {
	appID       int
	appHash     string
	sessionPath string
	ownerID     int64
	api         *tg.Client
}

func New(appID int, appHash string, sessionPath string, ownerID int64) *Client {
	return &Client{
		appID:       appID,
		appHash:     appHash,
		sessionPath: sessionPath,
		ownerID:     ownerID,
	}
}

// Run connects, authenticates (interactive terminal on first run), then calls
// handler for every incoming message until ctx is cancelled.
func (c *Client) Run(ctx context.Context, handler Handler) error {
	dispatcher := tg.NewUpdateDispatcher()

	tc := telegram.NewClient(c.appID, c.appHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: c.sessionPath},
		UpdateHandler:  dispatcher,
	})

	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		msg, ok := update.Message.(*tg.Message)
		if !ok {
			return nil
		}
		ev := c.buildEvent(msg, e)
		if ev == nil {
			return nil
		}
		handler(ctx, *ev)
		return nil
	})

	return tc.Run(ctx, func(ctx context.Context) error {
		c.api = tc.API()

		if err := auth.NewFlow(
			consoleAuth{},
			auth.SendCodeOptions{},
		).Run(ctx, tc.Auth()); err != nil {
			return fmt.Errorf("auth: %w", err)
		}

		log.Println("feedwatch: authenticated, listening for messages")
		<-ctx.Done()
		return ctx.Err()
	})
}

// SendMessage sends a text message to the given chat.
// When chatID equals ownerID (Saved Messages), uses InputPeerSelf to avoid
// needing an access hash for the user's own account.
func (c *Client) SendMessage(ctx context.Context, chatID int64, accessHash int64, text string) error {
	var peer tg.InputPeerClass
	if chatID == c.ownerID {
		peer = &tg.InputPeerSelf{}
	} else {
		peer = idToPeer(chatID, accessHash)
	}
	_, err := c.api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:      peer,
		Message:   text,
		RandomID:  randomID(),
		NoWebpage: true,
	})
	return err
}

// ForwardMessage forwards a message from one peer to another.
func (c *Client) ForwardMessage(ctx context.Context, from pipeline.Peer, msgID int, to pipeline.Peer) error {
	_, err := c.api.MessagesForwardMessages(ctx, &tg.MessagesForwardMessagesRequest{
		FromPeer: peerToInput(from),
		ID:       []int{msgID},
		ToPeer:   peerToInput(to),
		RandomID: []int64{randomID()},
		Silent:   true,
	})
	return err
}

// ResolvePeer resolves a Telegram username (with or without @) to a Peer.
// The returned Peer contains the MTProto ID and access hash needed for API calls.
func (c *Client) ResolvePeer(ctx context.Context, username string) (pipeline.Peer, error) {
	if len(username) > 0 && username[0] == '@' {
		username = username[1:]
	}
	resolved, err := c.api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		return pipeline.Peer{}, fmt.Errorf("resolve %q: %w", username, err)
	}
	switch p := resolved.Peer.(type) {
	case *tg.PeerChannel:
		for _, chat := range resolved.Chats {
			if ch, ok := chat.(*tg.Channel); ok && ch.ID == p.ChannelID {
				return pipeline.Peer{ID: ch.ID, AccessHash: ch.AccessHash, Type: "channel"}, nil
			}
		}
	case *tg.PeerUser:
		for _, u := range resolved.Users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				return pipeline.Peer{ID: user.ID, AccessHash: user.AccessHash, Type: "user"}, nil
			}
		}
	case *tg.PeerChat:
		for _, chat := range resolved.Chats {
			if ch, ok := chat.(*tg.Chat); ok && ch.ID == p.ChatID {
				return pipeline.Peer{ID: -ch.ID, Type: "chat"}, nil
			}
		}
	}
	return pipeline.Peer{}, fmt.Errorf("peer not found in response for %q", username)
}

func (c *Client) buildEvent(msg *tg.Message, e tg.Entities) *MessageEvent {
	ev := &MessageEvent{
		MessageID: msg.ID,
		Text:      msg.Message,
	}

	switch p := msg.PeerID.(type) {
	case *tg.PeerChannel:
		ev.ChatID = p.ChannelID
		ev.ChatType = "channel"
		if ch, ok := e.Channels[p.ChannelID]; ok {
			ev.ChatAccessHash = ch.AccessHash
		}
	case *tg.PeerChat:
		ev.ChatID = -p.ChatID
		ev.ChatType = "chat"
	case *tg.PeerUser:
		ev.ChatID = p.UserID
		ev.ChatType = "user"
	}

	if msg.Out {
		// Only care about outgoing messages to Saved Messages (self-commands).
		if p, ok := msg.PeerID.(*tg.PeerUser); ok && p.UserID == c.ownerID {
			ev.SenderID = c.ownerID
			ev.IsOwnerCommand = true
		} else {
			return nil
		}
	} else {
		if p, ok := msg.FromID.(*tg.PeerUser); ok {
			ev.SenderID = p.UserID
			ev.IsOwnerCommand = (p.UserID == c.ownerID)
		}
	}

	return ev
}

// idToPeer converts a chat ID and access hash to an InputPeer for groups and users.
func idToPeer(id int64, accessHash int64) tg.InputPeerClass {
	if id < 0 {
		return &tg.InputPeerChat{ChatID: -id}
	}
	return &tg.InputPeerUser{UserID: id, AccessHash: accessHash}
}

func peerToInput(p pipeline.Peer) tg.InputPeerClass {
	switch p.Type {
	case "chat":
		return &tg.InputPeerChat{ChatID: -p.ID}
	case "user":
		return &tg.InputPeerUser{UserID: p.ID, AccessHash: p.AccessHash}
	default: // "channel"
		return &tg.InputPeerChannel{ChannelID: p.ID, AccessHash: p.AccessHash}
	}
}

func randomID() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return int64(binary.LittleEndian.Uint64(b[:]))
}

// consoleAuth implements auth.UserAuthenticator via stdin prompts.
type consoleAuth struct{}

func (consoleAuth) Phone(_ context.Context) (string, error) {
	return prompt("Phone number (+...): ")
}

func (consoleAuth) Password(_ context.Context) (string, error) {
	return prompt("2FA password: ")
}

func (consoleAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	return prompt("Auth code: ")
}

func (consoleAuth) AcceptTermsOfService(_ context.Context, _ tg.HelpTermsOfService) error {
	return nil
}

func (consoleAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign up not supported: register via the official Telegram app first")
}

func prompt(label string) (string, error) {
	fmt.Print(label)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	return strings.TrimSpace(line), err
}
