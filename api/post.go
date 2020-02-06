package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rainyportrait/beldre/conf"
)

// GetPost - GET /api/v1/post/:id
func GetPost(router *gin.RouterGroup, conf *conf.Config) {
	router.GET("/post/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		if id == 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "invalid id",
			})
			return
		}

		sqlStr, _ := conf.GetTemplateString("select_post", nil)
		p := post{}
		err := conf.DB.Get(&p, sqlStr, id)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		sqlStr, _ = conf.GetTemplateString("select_post_tags", nil)
		t := []tag{}
		err = conf.DB.Select(&t, sqlStr, p.ID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"post": p,
			"tags": t,
		})
	})
}

type post struct {
	ID uint64 `json:"id"`

	Source       *string `json:"source,omitempty"`
	Uploader     uint64  `json:"uploader"`
	UploaderName string  `db:"uploader_name" json:"uploaderName" `
	Hash         string  `json:"hash"`

	CreatedAt string  `db:"created_at" json:"createdAt"`
	UpdatedAt *string `db:"updated_at" json:"updatedAt,omitempty"`
}

type tag struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
