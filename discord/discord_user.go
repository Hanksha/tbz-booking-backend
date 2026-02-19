package discord

type DiscordUser struct {
	ID   string `json:"userId"`
	Username string `json:"username"`
	Admin    bool   `json:"admin"`
}