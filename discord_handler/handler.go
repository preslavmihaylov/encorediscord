package discord_handler

import (
	"io"
	"net/http"

	"encore.dev/rlog"
)

// Webhook receives incoming webhooks from Some Service That Sends Webhooks.
//
//encore:api public raw method=POST path=/discord-webhook
func DiscordWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	rlog.Info("Received webhook", "body", string(body))
	w.WriteHeader(http.StatusOK)
}
