package discord_handler

import (
	"encoding/json"
	"io"
	"net/http"

	"encore.app/models"
	"encore.dev/rlog"
)

var secrets struct {
	DiscordPublicKey          string
	DiscordHandlerSecretToken string
}

// Webhook receives incoming webhooks from Some Service That Sends Webhooks.
//
//encore:api public raw method=POST path=/discord-webhook
func DiscordWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+secrets.DiscordHandlerSecretToken {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	var discordMsgEvent models.DiscordRawMessage
	if err := json.Unmarshal(body, &discordMsgEvent); err != nil {
		http.Error(w, "Error unmarshalling request body", http.StatusInternalServerError)
		return
	}

	rlog.Info("Received raw discord message", "discordMessage", discordMsgEvent)
	_, err = DiscordRawMessageTopic.Publish(r.Context(), &discordMsgEvent)
	if err != nil {
		http.Error(w, "Error publishing message", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
