package main

import (
	"html/template"

	"github.com/gin-gonic/gin"

	"github.com/rainyportrait/beldre/api"
	"github.com/rainyportrait/beldre/conf"
)

var sqlTmpl *template.Template

func main() {
	c := &conf.Config{}
	c.LoadEnv()
	c.LoadTemplates()
	c.ConnectToDatabase()

	r := gin.Default()
	v1 := r.Group("/api/v1")
	{
		api.GetPosts(v1, c)
		api.GetPost(v1, c)

		api.RegisterUser(v1, c)
		api.LoginUser(v1, c)
	}

	r.Run("localhost:8080")
	/*tags := []string{
		"fireboxstudio",
		"liang_xing",
		"zumi",
		"hydrafxx",
		"logan_cure",
		"rawrden",
		"vgerotica",
		"fpsblyck",
		"yeero",
		"cakeofcakes",
		"prywinko",
		"arhoangel",
		"tarakanovich",
	}*/
}
