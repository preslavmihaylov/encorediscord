package forumpostclassifier

import (
	"encore.app/models"
	"encore.dev/pubsub"
)

// UniqueDiscordForumPostTopic is a pubsub topic for forum posts
// classified as unique, meaning they didn't match any of the existing posts in our database
var UniqueDiscordForumPostTopic = pubsub.NewTopic[*models.DiscordForumPostEvent]("uniq-discord-forum-posts", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})

// DuplicateDiscordForumPostTopic is a pubsub topic for forum posts
// classified as duplicate, meaning they matched at least one existing post in our database
var DuplicateDiscordForumPostTopic = pubsub.NewTopic[*models.DuplicateDiscordForumPostEvent]("dup-discord-forum-posts", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})
