package response

import "github.com/gin-gonic/gin"

func OK(c *gin.Context, data any) {
	c.JSON(200, gin.H{"data": data})
}

func Created(c *gin.Context, data any) {
	c.JSON(201, gin.H{"data": data})
}

func NoContent(c *gin.Context) {
	c.Status(204)
}

func Error(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
