package api

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/hanksha/tbz-booking-system-backend/discord"
)

func DiscordAuth(discordClient discord.DiscordClient, adminRoleID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		accessToken := c.GetHeader("accesstoken")

		if len(accessToken) == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authentication"})
			c.Abort()
			return
		}

		member, err := discordClient.GetGuildMember(c.Request.Context(), accessToken)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authentication"})
			c.Abort()
			return
		}

		c.Set("user", discord.DiscordUser{
			ID:       member.User.ID,
			Username: member.User.Username,
			Admin:    slices.Contains(member.Roles, adminRoleID),
		})
		c.Set("accessToken", accessToken)
	}
}
