package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hanksha/tbz-booking-system-backend/discord"
)

type DiscordHandler struct {
	client      discord.DiscordClient
	adminRoleID string
}

func NewDiscordHandler(client discord.DiscordClient, adminRoleID string) *DiscordHandler {
	return &DiscordHandler{
		client:      client,
		adminRoleID: adminRoleID,
	}
}

func (h *DiscordHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/user/info", DiscordAuth(h.client, h.adminRoleID), h.GetUserInfo)
	rg.GET("/user/search", h.SearchUsers)
	rg.GET("/oauth/callback", h.OAuthCallback)
}

func (h *DiscordHandler) GetUserInfo(c *gin.Context) {
	user := c.MustGet("user").(discord.DiscordUser)

	c.IndentedJSON(http.StatusOK, user)
}

func (h *DiscordHandler) SearchUsers(c *gin.Context) {
	query := c.Query("query")
	query = strings.TrimSpace(query)

	if len(query) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query cannot be empty"})
		return
	}

	members, err := h.client.SearchMembers(c.Request.Context(), query, 20)

	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search users"})
		return
	}

	usernames := []string{}

	for _, member := range members {
		usernames = append(usernames, member.User.Username)
	}

	c.IndentedJSON(http.StatusOK, usernames)
}

func (h *DiscordHandler) OAuthCallback(c *gin.Context) {
	code := c.Query("code")
	token, err := h.client.GetOAuth2Token(c.Request.Context(), code)

	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get oauth2 token"})
		return
	}

	c.IndentedJSON(http.StatusOK, token)
}
