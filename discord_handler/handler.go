package discord_handler

import (
	"encoding/json"
	"io"
	"net/http"

	"encore.dev/rlog"
	"github.com/bwmarrin/discordgo"
)

var secrets struct {
	DiscordPublicKey string
}

type UnknownDiscordInteraction struct {
	InteractionType discordgo.InteractionType `json:"type"`
}

// Webhook receives incoming webhooks from Some Service That Sends Webhooks.
//
//encore:api public raw method=POST path=/discord-webhook
func DiscordWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// publicKeyBytes, err := hex.DecodeString(secrets.DiscordPublicKey)
	// if err != nil {
	// 	http.Error(w, "Error decoding hex string", http.StatusInternalServerError)
	// 	return
	// }

	// if !discordgo.VerifyInteraction(r, publicKeyBytes) {
	// 	http.Error(w, "invalid request signature", http.StatusUnauthorized)
	// 	return
	// }

	rlog.Info("Received webhook", "body", string(body))
	var interaction UnknownDiscordInteraction
	if err := json.Unmarshal(body, &interaction); err != nil {
		http.Error(w, "Error unmarshalling request body", http.StatusInternalServerError)
		return
	}

	if interaction.InteractionType == discordgo.InteractionPing {
		pongResp, err := json.Marshal(discordgo.InteractionResponse{
			Type: discordgo.InteractionResponsePong,
		})
		if err != nil {
			http.Error(w, "Error marshalling response", http.StatusInternalServerError)
			return
		}

		// set content-type header
		w.Header().Set("Content-Type", "application/json")
		w.Write(pongResp)
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
}
